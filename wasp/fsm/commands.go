package fsm

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	packet "github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/cluster/raft"
)

type State interface {
	Subscribe(peer uint64, id string, pattern []byte, qos int32) error
	Unsubscribe(id string, pattern []byte) error
	RemoveSubscriptionsForPeer(peer uint64)
	RemoveSubscriptionsForSession(id string)
	RetainMessage(msg *packet.Publish) error
	DeleteRetainedMessage(topic []byte) error
	CreateSessionMetadata(id string, peer uint64, clientID string, connectedAt int64, lwt *packet.Publish) error
	DeleteSessionMetadata(id string, peer uint64) error
	DeleteSessionMetadatasByPeer(peer uint64)
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
	return f.SubscribeFrom(ctx, id, f.id, pattern, qos)
}
func (f *FSM) SubscribeFrom(ctx context.Context, id string, peer uint64, pattern []byte, qos int32) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_SessionSubscribed{
		SessionSubscribed: &SubscriptionCreated{
			SessionID: id,
			Pattern:   pattern,
			Qos:       qos,
			Peer:      peer,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) CreateSessionMetadata(ctx context.Context, id, clientID string, lwt *packet.Publish) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_SessionCreated{
		SessionCreated: &SessionCreated{
			SessionID:   id,
			ClientID:    clientID,
			ConnectedAt: time.Now().Unix(),
			Peer:        f.id,
			LWT:         lwt,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) DeleteSessionMetadata(ctx context.Context, id string) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_SessionDeleted{
		SessionDeleted: &SessionDeleted{
			SessionID: id,
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
			f.state.DeleteSessionMetadatasByPeer(input.Peer)
		case *StateTransition_SessionCreated:
			input := event.SessionCreated
			f.state.CreateSessionMetadata(input.SessionID, input.Peer, input.ClientID, input.ConnectedAt, input.LWT)
		case *StateTransition_SessionDeleted:
			input := event.SessionDeleted
			f.state.DeleteSessionMetadata(input.SessionID, input.Peer)
			f.state.RemoveSubscriptionsForSession(input.SessionID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
