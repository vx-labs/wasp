package distributed

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/wasp/crdt"
	"github.com/vx-labs/wasp/subscriptions"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/audit"
)

type subscriptionsState struct {
	mu            sync.RWMutex
	peer          uint64
	subscriptions subscriptions.Tree
	bcast         *memberlist.TransmitLimitedQueue
	recorder      audit.Recorder
}

func newSubscriptionState(peer uint64, bcast *memberlist.TransmitLimitedQueue, recorder audit.Recorder) *subscriptionsState {
	return &subscriptionsState{
		subscriptions: subscriptions.NewTree(),
		peer:          peer,
		bcast:         bcast,
		recorder:      recorder,
	}
}

func (s *subscriptionsState) mergeSubscriptions(subscriptions []*api.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, subscription := range subscriptions {
		if subscription.SessionID == "" || len(subscription.Pattern) == 0 {
			return ErrInvalidPayload
		}
		s.set(*subscription)
	}
	return nil
}

func (s *subscriptionsState) dump(event *api.StateBroadcastEvent) {
	subscriptions := s.All()
	for _, subscription := range subscriptions {
		event.Subscriptions = append(event.Subscriptions, &subscription)
	}
}

func (s *subscriptionsState) Create(sessionID string, pattern []byte, qos int32) error {
	return s.CreateFrom(sessionID, s.peer, pattern, qos)
}
func (s *subscriptionsState) CreateFrom(sessionID string, peer uint64, pattern []byte, qos int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	subscription := api.Subscription{
		SessionID: sessionID,
		Pattern:   pattern,
		Peer:      s.peer,
		QoS:       qos,
		LastAdded: clock(),
	}
	s.set(subscription)
	buf, err := proto.Marshal(&api.StateBroadcastEvent{
		Subscriptions: []*api.Subscription{&subscription},
	})
	if err != nil {
		return err
	}
	mountpointIdx := bytes.Index(pattern, []byte{'/'})
	s.recorder.RecordEvent(string(pattern[:mountpointIdx]), audit.SubscriptionCreated, map[string]string{
		"session_id": sessionID,
		"pattern":    string(pattern),
		"qos":        fmt.Sprintf("%d", qos),
	})
	s.bcast.QueueBroadcast(simpleBroadcast(buf))
	return nil
}
func (s *subscriptionsState) Delete(sessionID string, pattern []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	subscription := api.Subscription{
		SessionID:   sessionID,
		Pattern:     pattern,
		Peer:        s.peer,
		LastDeleted: clock(),
	}
	s.set(subscription)
	buf, err := proto.Marshal(&api.StateBroadcastEvent{
		Subscriptions: []*api.Subscription{&subscription},
	})
	if err != nil {
		return err
	}
	mountpointIdx := bytes.Index(pattern, []byte{'/'})
	s.recorder.RecordEvent(string(pattern[:mountpointIdx]), audit.SubscriptionDeleted, map[string]string{
		"session_id": sessionID,
		"pattern":    string(pattern),
	})
	s.bcast.QueueBroadcast(simpleBroadcast(buf))
	return nil
}
func (s *subscriptionsState) DeletePeer(peer uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := clock()
	toDelete := s.filter(func(a api.Subscription) bool { return a.Peer == peer })
	event := &api.StateBroadcastEvent{Subscriptions: []*api.Subscription{}}

	for _, subscription := range toDelete {
		subscription.LastDeleted = now
		s.set(subscription)
		event.Subscriptions = append(event.Subscriptions, &subscription)
	}
	buf, err := proto.Marshal(event)
	if err != nil {
		return
	}
	s.bcast.QueueBroadcast(simpleBroadcast(buf))
}
func (s *subscriptionsState) DeleteSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := clock()
	toDelete := s.filter(func(a api.Subscription) bool { return a.SessionID == id })
	event := &api.StateBroadcastEvent{Subscriptions: []*api.Subscription{}}

	for _, subscription := range toDelete {
		subscription.LastDeleted = now
		s.set(subscription)
		event.Subscriptions = append(event.Subscriptions, &subscription)
	}
	buf, err := proto.Marshal(event)
	if err != nil {
		return
	}
	s.bcast.QueueBroadcast(simpleBroadcast(buf))
}

func (s *subscriptionsState) ByPeer(peer uint64) []api.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filter(func(a api.Subscription) bool { return a.Peer == peer })
}
func (s *subscriptionsState) ByPattern(pattern []byte) []api.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filterPattern(pattern, func(a api.Subscription) bool { return true })
}
func (s *subscriptionsState) All() []api.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filter(func(a api.Subscription) bool { return true })
}

func (s *subscriptionsState) filter(f func(api.Subscription) bool) []api.Subscription {
	out := make([]api.Subscription, 0)
	s.subscriptions.Iterate(func(b []byte) {
		local := &api.SubscriptionList{}
		err := proto.Unmarshal(b, local)
		if err == nil {
			for _, sub := range local.Subscriptions {
				if crdt.IsEntryAdded(sub) && f(*sub) {
					out = append(out, *sub)
				}
			}
		}
	})
	return out
}
func (s *subscriptionsState) filterPattern(pattern []byte, f func(api.Subscription) bool) []api.Subscription {
	out := make([]api.Subscription, 0)
	s.subscriptions.Walk(pattern, func(b []byte) {
		local := &api.SubscriptionList{}
		err := proto.Unmarshal(b, local)
		if err == nil {
			for _, sub := range local.Subscriptions {
				if crdt.IsEntryAdded(sub) && f(*sub) {
					out = append(out, *sub)
				}
			}
		}
	})
	return out
}
func (s *subscriptionsState) set(subscription api.Subscription) {
	s.subscriptions.Upsert(subscription.Pattern, func(b []byte) []byte {
		local := &api.SubscriptionList{}
		err := proto.Unmarshal(b, local)
		if err != nil {
			// corrupted data, erase it
			buf, err := proto.Marshal(&api.SubscriptionList{
				Subscriptions: []*api.Subscription{
					&subscription,
				},
			})
			if err != nil {
				return nil
			}
			return buf
		}

		found := false
		for idx, sub := range local.Subscriptions {
			if sub.SessionID == subscription.SessionID {
				found = true
				if crdt.IsEntryOutdated(sub, &subscription) {
					local.Subscriptions[idx] = &subscription
					break
				}
			}
		}
		if !found {
			local.Subscriptions = append(local.Subscriptions, &subscription)
		}
		buf, err := proto.Marshal(local)
		if err != nil {
			return nil
		}
		return buf
	})
}
