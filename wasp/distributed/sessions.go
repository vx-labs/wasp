package distributed

import (
	"errors"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/crdt"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/audit"
	"github.com/vx-labs/wasp/wasp/stats"
)

type sessionMetadatasState struct {
	mu       sync.RWMutex
	peer     uint64
	sessions map[string]api.SessionMetadatas
	bcast    *memberlist.TransmitLimitedQueue
	recorder audit.Recorder
}

var (
	ErrSessionMetadatasNotFound = errors.New("session metadatas not found")
)

func newSessionMetadatasState(peer uint64, bcast *memberlist.TransmitLimitedQueue, recorder audit.Recorder) *sessionMetadatasState {
	return &sessionMetadatasState{
		sessions: make(map[string]api.SessionMetadatas),
		peer:     peer,
		bcast:    bcast,
		recorder: recorder,
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
func (s *sessionMetadatasState) dump(event *api.StateBroadcastEvent) {
	sessions := s.All()
	for _, session := range sessions {
		event.SessionMetadatas = append(event.SessionMetadatas, &session)
	}
}
func (s *sessionMetadatasState) Create(id string, clientID string, connectedAt int64, lwt *packet.Publish, mountpoint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if ok && crdt.IsEntryAdded(&session) {
		return ErrSessionMetadatasExists
	}
	session = api.SessionMetadatas{
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
	s.recorder.RecordEvent(mountpoint, audit.SessionConnected, map[string]string{
		"session_id": id,
		"client_id":  clientID,
	})
	stats.SessionsCount.Inc()
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
	session, ok := s.sessions[id]
	if !ok || crdt.IsEntryRemoved(&session) {
		return nil
	}
	session.LastDeleted = clock()
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
	s.recorder.RecordEvent(session.MountPoint, audit.SessionDisonnected, map[string]string{
		"session_id": id,
	})
	stats.SessionsCount.Dec()
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
func (s *sessionMetadatasState) ByPeer(peer uint64) []api.SessionMetadatas {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filter(func(s api.SessionMetadatas) bool { return s.Peer == peer })
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
