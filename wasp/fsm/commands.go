package fsm

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	packet "github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/raft"
)

type State interface {
	Subscribe(id string, pattern []byte, qos int32) error
	Unsubscribe(id string, pattern []byte) error
	RetainMessage(msg *packet.Publish) error
	DeleteRetainedMessage(topic []byte) error
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

func NewFSM(id string, state State, commandsCh chan raft.Command) *FSM {
	return &FSM{state: state, commandsCh: commandsCh}
}

type FSM struct {
	id         string
	state      State
	commandsCh chan raft.Command
}

func (f *FSM) commit(ctx context.Context, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out := make(chan error)
	defer close(out)
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
			err = f.state.Subscribe(input.SessionID, input.Pattern, input.Qos)
		case *StateTransition_SessionUnsubscribed:
			input := event.SessionUnsubscribed
			err = f.state.Unsubscribe(input.SessionID, input.Pattern)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
