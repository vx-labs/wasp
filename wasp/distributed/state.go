package distributed

import (
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
)

var (
	clock = func() int64 {
		return time.Now().UnixNano()
	}
)

var (
	ErrInvalidPayload = errors.New("invalid payload")
)

type Channel interface {
	Events() chan []byte
	Broadcast(b []byte)
	BroadcastFullState(b []byte)
}

type SubscriptionsState interface {
	Create(sessionID string, pattern []byte, qos int32) error
	CreateFrom(sessionID string, peer uint64, pattern []byte, qos int32) error
	All() []api.Subscription
	ByPattern(pattern []byte) []api.Subscription
	ByPeer(peer uint64) []api.Subscription
	Delete(sessionID string, pattern []byte) error
	DeletePeer(peer uint64)
	DeleteSession(sessionID string)
}
type SessionMetadatasState interface {
	Create(id string, clientID string, connectedAt int64, lwt *packet.Publish, mountpoint string) error
	Get(id string) (api.SessionMetadatas, error)
	ByClientID(clientID string) (api.SessionMetadatas, error)
	ByPeer(peer uint64) []api.SessionMetadatas
	All() []api.SessionMetadatas
	Delete(id string) error
	DeletePeer(peer uint64) error
}
type TopicsState interface {
	Set(message *packet.Publish) error
	Delete(topic []byte) error
	Get(pattern []byte) ([]*packet.Publish, error)
}

type State interface {
	Subscriptions() SubscriptionsState
	SessionMetadatas() SessionMetadatasState
	Topics() TopicsState
	Distributor() memberlist.Delegate
}

type state struct {
	bcast            *memberlist.TransmitLimitedQueue
	topics           *topicsState
	subscriptions    *subscriptionsState
	sessionMetadatas *sessionMetadatasState
}

func NewState(peer uint64, bcast *memberlist.TransmitLimitedQueue) State {
	return &state{
		bcast:            bcast,
		topics:           newTopicState(bcast),
		sessionMetadatas: newSessionMetadatasState(peer, bcast),
		subscriptions:    newSubscriptionState(peer, bcast),
	}
}

func (s *state) Distributor() memberlist.Delegate        { return s }
func (s *state) Subscriptions() SubscriptionsState       { return s.subscriptions }
func (s *state) SessionMetadatas() SessionMetadatasState { return s.sessionMetadatas }
func (s *state) Topics() TopicsState                     { return s.topics }
func (s *state) GetBroadcasts(overhead, limit int) [][]byte {
	return s.bcast.GetBroadcasts(overhead, limit)
}
func (s *state) MergeRemoteState(buf []byte, join bool) {
	payload := &api.StateBroadcastEvent{}
	err := proto.Unmarshal(buf, payload)
	if err != nil {
		return
	}
	s.sessionMetadatas.mergeSessions(payload.SessionMetadatas)
	s.subscriptions.mergeSubscriptions(payload.Subscriptions)
	s.topics.mergeMessages(payload.RetainedMessages)
}
func (s *state) NotifyMsg(b []byte) {
	s.MergeRemoteState(b, false)
}
func (s *state) NodeMeta(limit int) []byte { return nil }
func (s *state) LocalState(join bool) []byte {
	payload := &api.StateBroadcastEvent{}

	s.subscriptions.dump(payload)
	s.sessionMetadatas.dump(payload)
	s.topics.dump(payload)

	buf, err := proto.Marshal(payload)
	if err == nil {
		return buf
	}
	return nil
}
