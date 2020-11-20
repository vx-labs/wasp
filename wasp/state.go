package wasp

import (
	"sync"

	"github.com/vx-labs/wasp/v4/wasp/sessions"
	"github.com/zond/gotomic"
)

type LocalState interface {
	Get(id string) *sessions.Session
	ListSessions() []*sessions.Session
	Create(id string, session *sessions.Session) *sessions.Session
	Delete(id string) *sessions.Session
}

type hashmapState struct {
	id       uint64
	sessions *gotomic.Hash
}

func NewState(id uint64) LocalState {
	return newLockedMapState(id)
}

func newLockedMapState(id uint64) LocalState {
	return &lockedMapState{id: id, sessions: map[string]*sessions.Session{}}
}

type lockedMapState struct {
	id       uint64
	sessions map[string]*sessions.Session
	mtx      sync.RWMutex
}

func (s *lockedMapState) Get(id string) *sessions.Session {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.sessions[id]
}
func (s *lockedMapState) ListSessions() []*sessions.Session {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	out := []*sessions.Session{}
	for _, s := range s.sessions {
		out = append(out, s)
	}
	return out
}
func (s *lockedMapState) Create(id string, session *sessions.Session) *sessions.Session {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	old := s.sessions[id]
	s.sessions[id] = session
	return old
}
func (s *lockedMapState) Delete(id string) *sessions.Session {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	old := s.sessions[id]
	delete(s.sessions, id)
	return old
}
