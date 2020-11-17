package wasp

import (
	"sync"

	"github.com/vx-labs/wasp/wasp/sessions"
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

func newHashmapState(id uint64) LocalState {
	return &hashmapState{id: id, sessions: gotomic.NewHash()}
}
func newLockedMapState(id uint64) LocalState {
	return &lockedMapState{id: id, sessions: map[string]*sessions.Session{}}
}

func (s *hashmapState) Get(id string) *sessions.Session {
	v, ok := s.sessions.Get(gotomic.StringKey(id))
	if !ok {
		return nil
	}
	return v.(*sessions.Session)
}
func (s *hashmapState) ListSessions() []*sessions.Session {
	out := []*sessions.Session{}
	s.sessions.Each(func(k gotomic.Hashable, v gotomic.Thing) bool {
		out = append(out, v.(*sessions.Session))
		return false
	})
	return out
}
func (s *hashmapState) Create(id string, session *sessions.Session) *sessions.Session {
	old, ok := s.sessions.Put(gotomic.StringKey(id), session)
	if ok {
		return old.(*sessions.Session)
	}
	return nil
}
func (s *hashmapState) Delete(id string) *sessions.Session {
	old, ok := s.sessions.Delete(gotomic.StringKey(id))
	if ok {
		return old.(*sessions.Session)
	}
	return nil
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
