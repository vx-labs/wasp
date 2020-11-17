package wasp

import (
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
	return &hashmapState{
		id:       id,
		sessions: gotomic.NewHash(),
	}
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
