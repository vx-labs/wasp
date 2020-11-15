package distributed

import (
	"errors"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/crdt"
	"github.com/vx-labs/wasp/wasp/api"
)

type sessionMetadatasState struct {
	mu       sync.RWMutex
	peer     uint64
	sessions map[string]api.SessionMetadatas
	bcast    *memberlist.TransmitLimitedQueue
}

var (
	ErrSessionMetadatasNotFound = errors.New("session metadatas not found")
)

func newSessionMetadatasState(peer uint64, bcast *memberlist.TransmitLimitedQueue) SessionMetadatasState {
	return &sessionMetadatasState{
		sessions: make(map[string]api.SessionMetadatas),
		peer:     peer,
		bcast:    bcast,
	}
}
func (s *sessionMetadatasState) mergeSessions(sessions []*api.SessionMetadatas) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range sessions {
		if session.SessionID == "" {
			return ErrInvalidPayload
		}

		local, ok := s.sessions[session.SessionID]
		outdated := !ok || crdt.IsEntryOutdated(&local, session)

		if outdated {
			s.sessions[session.SessionID] = *session
		}
	}
	return nil
}

func (s *sessionMetadatasState) GetBroadcasts(overhead, limit int) [][]byte {
	return s.bcast.GetBroadcasts(overhead, limit)
}

func (s *sessionMetadatasState) Merge(buf []byte) error {
	payload := &api.StateBroadcastEvent{}
	err := proto.Unmarshal(buf, payload)
	if err != nil {
		return err
	}
	return s.mergeSessions(payload.SessionMetadatas)
}

func (s *sessionMetadatasState) Create(id string, clientID string, connectedAt int64, lwt *packet.Publish, mountpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := api.SessionMetadatas{
		SessionID:   id,
		ClientID:    clientID,
		ConnectedAt: connectedAt,
		LWT:         lwt,
		MountPoint:  mountpoint,
		Peer:        s.peer,
		LastAdded:   clock(),
	}
	err := s.set(session)
	if err != nil {
		return err
	}
	buf, err := proto.Marshal(&api.StateBroadcastEvent{
		SessionMetadatas: []*api.SessionMetadatas{&session},
	})
	if err != nil {
		return err
	}
	s.bcast.QueueBroadcast(simpleBroadcast(buf))
	return nil
}
func (s *sessionMetadatasState) set(session api.SessionMetadatas) error {
	s.sessions[session.SessionID] = session
	return nil
}

func (s *sessionMetadatasState) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := api.SessionMetadatas{
		SessionID:   id,
		Peer:        s.peer,
		LastDeleted: clock(),
	}
	err := s.set(session)
	if err != nil {
		return err
	}
	buf, err := proto.Marshal(&api.StateBroadcastEvent{
		SessionMetadatas: []*api.SessionMetadatas{&session},
	})
	if err != nil {
		return err
	}
	s.bcast.QueueBroadcast(simpleBroadcast(buf))
	return nil
}

func (s *sessionMetadatasState) Get(id string) (api.SessionMetadatas, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[id]
	if !ok || !crdt.IsEntryAdded(&v) {
		return api.SessionMetadatas{}, ErrSessionMetadatasNotFound
	}
	return v, nil
}

func (s *sessionMetadatasState) ByClientID(clientID string) (api.SessionMetadatas, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.find(func(s api.SessionMetadatas) bool { return s.ClientID == clientID })
}
func (s *sessionMetadatasState) All() []api.SessionMetadatas {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filter(func(s api.SessionMetadatas) bool { return true })
}
func (s *sessionMetadatasState) filter(f func(s api.SessionMetadatas) bool) []api.SessionMetadatas {
	c := 0
	for _, md := range s.sessions {
		if crdt.IsEntryAdded(&md) && f(md) {
			c++
		}
	}

	out := make([]api.SessionMetadatas, c)
	idx := 0
	for _, md := range s.sessions {
		if crdt.IsEntryAdded(&md) && f(md) {
			out[idx] = md
			idx++
		}
	}
	return out
}
func (s *sessionMetadatasState) find(f func(s api.SessionMetadatas) bool) (api.SessionMetadatas, error) {
	for _, md := range s.sessions {
		if crdt.IsEntryAdded(&md) && f(md) {
			return md, nil
		}
	}
	return api.SessionMetadatas{}, ErrSessionMetadatasNotFound
}

func (s *sessionMetadatasState) DeletePeer(peer uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions := s.filter(func(s api.SessionMetadatas) bool { return s.Peer == peer })

	event := &api.StateBroadcastEvent{SessionMetadatas: []*api.SessionMetadatas{}}
	for _, session := range sessions {
		session.LastDeleted = clock()
		event.SessionMetadatas = append(event.SessionMetadatas, &session)
		s.set(session)
	}

	buf, err := proto.Marshal(event)
	if err != nil {
		return err
	}
	s.bcast.QueueBroadcast(simpleBroadcast(buf))
	return nil
}
