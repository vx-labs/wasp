package fsm

import (
	"bytes"
	"context"
	fmt "fmt"
	"log"
	"time"

	"github.com/gogo/protobuf/proto"
	packet "github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/cluster/raft"
	"github.com/vx-labs/wasp/wasp/audit"
	"go.etcd.io/etcd/raft/raftpb"
)

type State interface {
	Subscribe(peer uint64, id string, pattern []byte, qos int32) error
	Unsubscribe(id string, pattern []byte) error
	RemoveSubscriptionsForPeer(peer uint64)
	RemoveSubscriptionsForSession(id string)
	RetainMessage(msg *packet.Publish) error
	DeleteRetainedMessage(topic []byte) error
	CreateSessionMetadata(id string, peer uint64, clientID string, connectedAt int64, lwt *packet.Publish, mountpoint string) error
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

func NewFSM(id uint64, state State, commandsCh chan raft.Command, recorder audit.Recorder) *FSM {
	return &FSM{id: id, state: state, commandsCh: commandsCh, recorder: recorder}
}

type FSM struct {
	id         uint64
	state      State
	commandsCh chan raft.Command
	recorder   audit.Recorder
}

func splitTenant(topic []byte) (string, []byte) {
	idx := bytes.IndexRune(topic, '/')
	return string(topic[:idx]), topic[idx+1:]
}

func (f *FSM) record(ctx context.Context, events ...*StateTransition) error {
	var err error
	for _, event := range events {
		switch event := event.GetEvent().(type) {
		case *StateTransition_SessionSubscribed:
			input := event.SessionSubscribed
			tenant, topic := splitTenant(input.Pattern)
			err = f.recorder.RecordEvent(tenant, audit.SubscriptionCreated, map[string]string{
				"pattern":    string(topic),
				"qos":        fmt.Sprintf("%d", input.Qos),
				"session_id": input.SessionID,
			})
		case *StateTransition_SessionUnsubscribed:
			input := event.SessionUnsubscribed
			tenant, topic := splitTenant(input.Pattern)
			err = f.recorder.RecordEvent(tenant, audit.SubscriptionDeleted, map[string]string{
				"pattern":    string(topic),
				"session_id": input.SessionID,
			})
		case *StateTransition_PeerLost:
		case *StateTransition_SessionCreated:
			input := event.SessionCreated
			err = f.recorder.RecordEvent(input.MountPoint, audit.SessionConnected, map[string]string{
				"session_id": input.SessionID,
				"client_id":  input.ClientID,
			})
		case *StateTransition_SessionDeleted:
			input := event.SessionDeleted
			err = f.recorder.RecordEvent(input.MountPoint, audit.SessionDisonnected, map[string]string{
				"session_id": input.SessionID,
			})
		}
		if err != nil {
			log.Println(err)
			// Do not fail if audit recording fails
			return nil
		}
	}
	return nil
}
func (f *FSM) commit(ctx context.Context, events ...*StateTransition) error {
	payload, err := encode(events...)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
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
			if err == nil {
				f.record(ctx, events...)
			}
			return err
		}
	}
}
func (f *FSM) RetainedMessage(ctx context.Context, publish *packet.Publish) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_RetainedMessageStored{
		RetainedMessageStored: &RetainedMessageStored{
			Publish: publish,
		},
	}})
}
func (f *FSM) DeleteRetainedMessage(ctx context.Context, topic []byte) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_RetainedMessageDeleted{
		RetainedMessageDeleted: &RetainedMessageDeleted{
			Topic: topic,
		},
	}})
}

func (f *FSM) Subscribe(ctx context.Context, id string, pattern []byte, qos int32) error {
	return f.SubscribeFrom(ctx, id, f.id, pattern, qos)
}
func (f *FSM) SubscribeFrom(ctx context.Context, id string, peer uint64, pattern []byte, qos int32) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_SessionSubscribed{
		SessionSubscribed: &SubscriptionCreated{
			SessionID: id,
			Pattern:   pattern,
			Qos:       qos,
			Peer:      peer,
		},
	}})
}
func (f *FSM) CreateSessionMetadata(ctx context.Context, id, clientID string, lwt *packet.Publish, mountpoint string) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_SessionCreated{
		SessionCreated: &SessionCreated{
			SessionID:   id,
			ClientID:    clientID,
			ConnectedAt: time.Now().Unix(),
			Peer:        f.id,
			LWT:         lwt,
			MountPoint:  mountpoint,
		},
	}})
}
func (f *FSM) DeleteSessionMetadata(ctx context.Context, id, mountpoint string) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_SessionDeleted{
		SessionDeleted: &SessionDeleted{
			SessionID:  id,
			Peer:       f.id,
			MountPoint: mountpoint,
		},
	}})
}
func (f *FSM) Unsubscribe(ctx context.Context, id string, pattern []byte) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_SessionUnsubscribed{
		SessionUnsubscribed: &SubscriptionDeleted{
			SessionID: id,
			Pattern:   pattern,
		},
	}})
}
func (f *FSM) removePeer(id uint64) {
	f.state.DeleteSessionMetadatasByPeer(id)
	f.state.RemoveSubscriptionsForPeer(id)
}

func (f *FSM) ApplyConfChange(ctx context.Context, index uint64, cc raftpb.ConfChangeI) error {
	ccv1, ok := cc.AsV1()
	if ok {
		switch ccv1.Type {
		case raftpb.ConfChangeRemoveNode:
			f.removePeer(ccv1.NodeID)
		}
	} else {
		for _, change := range cc.AsV2().Changes {
			switch change.Type {
			case raftpb.ConfChangeRemoveNode:
				f.removePeer(change.NodeID)
			}
		}
	}
	return nil
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
			f.removePeer(input.Peer)
		case *StateTransition_SessionCreated:
			input := event.SessionCreated
			f.state.CreateSessionMetadata(input.SessionID, input.Peer, input.ClientID, input.ConnectedAt, input.LWT, input.MountPoint)
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
