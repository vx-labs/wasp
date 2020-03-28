package fsm

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	packet "github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/raft"
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/stats"
)

type State interface {
	Subscribe(peer uint64, id string, pattern []byte, qos int32) error
	Unsubscribe(id string, pattern []byte) error
	RemoveSubscriptionsForPeer(peer uint64)
	RetainMessage(msg *packet.Publish) error
	DeleteRetainedMessage(topic []byte) error
	Recipients(topic []byte) ([]uint64, []string, []int32, error)
	GetSession(id string) *sessions.Session
	SaveSession(id string, session *sessions.Session)
	CloseSession(id string)
	RetainedMessages(topic []byte) ([]*packet.Publish, error)
}

func decode(payload []byte) ([]*StateTransition, error) {
	format := StateTransitionSet{}
	err := proto.Unmarshal(payload, &format)
	if err != nil {
		return nil, err
	}
	return format.Events, nil
}
func encode(events ...*StateTransition) ([]byte, error) {
	format := StateTransitionSet{
		Events: events,
	}
	return proto.Marshal(&format)
}

func NewFSM(id uint64, state State, commandsCh chan raft.Command) *FSM {
	return &FSM{id: id, state: state, commandsCh: commandsCh}
}

type FSM struct {
	id         uint64
	state      State
	commandsCh chan raft.Command
}

func (f *FSM) commit(ctx context.Context, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out := make(chan error)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case f.commandsCh <- raft.Command{Ctx: ctx, ErrCh: out, Payload: payload}:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-out:
			return err
		}
	}
}
func (f *FSM) Shutdown(ctx context.Context) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_PeerLost{
		PeerLost: &PeerLost{
			Peer: f.id,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) RetainedMessage(ctx context.Context, publish *packet.Publish) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_RetainedMessageStored{
		RetainedMessageStored: &RetainedMessageStored{
			Publish: publish,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) DeleteRetainedMessage(ctx context.Context, topic []byte) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_RetainedMessageDeleted{
		RetainedMessageDeleted: &RetainedMessageDeleted{
			Topic: topic,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}

func (f *FSM) Subscribe(ctx context.Context, id string, pattern []byte, qos int32) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_SessionSubscribed{
		SessionSubscribed: &SubscriptionCreated{
			SessionID: id,
			Pattern:   pattern,
			Qos:       qos,
			Peer:      f.id,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) Unsubscribe(ctx context.Context, id string, pattern []byte) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_SessionUnsubscribed{
		SessionUnsubscribed: &SubscriptionDeleted{
			SessionID: id,
			Pattern:   pattern,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}

func (f *FSM) PublishMessages(ctx context.Context, messages []*packet.Publish) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_MessagesPublished{
		MessagesPublished: &MessagesPublished{
			Publishes: messages,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) Apply(b []byte) error {
	events, err := decode(b)
	if err != nil {
		return err
	}
	for _, event := range events {
		switch event := event.GetEvent().(type) {
		case *StateTransition_RetainedMessageDeleted:
			input := event.RetainedMessageDeleted
			err = f.state.DeleteRetainedMessage(input.Topic)
		case *StateTransition_RetainedMessageStored:
			input := event.RetainedMessageStored
			err = f.state.RetainMessage(input.Publish)
		case *StateTransition_SessionSubscribed:
			input := event.SessionSubscribed
			err = f.state.Subscribe(input.Peer, input.SessionID, input.Pattern, input.Qos)
		case *StateTransition_SessionUnsubscribed:
			input := event.SessionUnsubscribed
			err = f.state.Unsubscribe(input.SessionID, input.Pattern)
		case *StateTransition_PeerLost:
			input := event.PeerLost
			f.state.RemoveSubscriptionsForPeer(input.Peer)
		case *StateTransition_MessagesPublished:
			input := event.MessagesPublished
			for _, p := range input.Publishes {
				f.processPublish(p)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func getLowerQos(a, b int32) int32 {
	if a > b {
		return b
	}
	return a
}

func (f *FSM) processPublish(p *packet.Publish) error {
	start := time.Now()
	defer func() {
		duration := float64(time.Since(start)) / float64(time.Millisecond)
		stats.Histogram("publishProcessingTime").Observe(duration)
	}()
	if p.Header.Retain {
		if len(p.Payload) == 0 {
			err := f.state.DeleteRetainedMessage(p.Topic)
			if err != nil {
				return err
			}
		} else {
			err := f.state.RetainMessage(p)
			if err != nil {
				return err
			}
		}
		p.Header.Retain = false
	}
	peers, recipients, qoss, err := f.state.Recipients(p.Topic)
	if err != nil {
		return err
	}
	for idx := range recipients {
		publish := &packet.Publish{
			Header: &packet.Header{
				Dup: p.Header.Dup,
				Qos: getLowerQos(qoss[idx], p.Header.Qos),
			},
			Payload: p.Payload,
			Topic:   p.Topic,
		}
		if peers[idx] == f.id {
			session := f.state.GetSession(recipients[idx])
			if session != nil {
				session.Send(publish)
			}
		}
	}
	return nil
}
