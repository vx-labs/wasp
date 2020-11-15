package distributed

import (
	"errors"
	"time"

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
	All() []*api.Subscription
	ByPattern(pattern []byte) []*api.Subscription
	ByPeer(peer uint64) []*api.Subscription
	Delete(sessionID string, pattern []byte) error
	DeletePeer(peer uint64)
	DeleteSession(sessionID string)
}
type SessionMetadatasState interface {
	Create(id string, clientID string, connectedAt int64, lwt *packet.Publish, mountpoint string) error
	Get(id string) *api.SessionMetadatas
	ByClientID(clientID string) *api.SessionMetadatas
	All() []*api.SessionMetadatas
	Delete(id string) error
	DeletePeer(peer uint64)
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
}
