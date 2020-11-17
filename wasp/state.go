package wasp

import (
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/stats"
)

type LocalState interface {
	GetSession(id string) *sessions.Session
	ListSessions() []*sessions.Session
	SaveSession(id string, session *sessions.Session)
	CloseSession(id string)
}

type state struct {
	id       uint64
	sessions sessions.Store
}

func NewState(id uint64) LocalState {
	return &state{
		id:       id,
		sessions: sessions.NewStore(),
	}
}

func (s *state) GetSession(id string) *sessions.Session {
	return s.sessions.Get(id)
}
func (s *state) ListSessions() []*sessions.Session {
	return s.sessions.All()
}
func (s *state) SaveSession(id string, session *sessions.Session) {
	s.sessions.Save(id, session)
	stats.SessionsCount.Set(float64(s.sessions.Count()))
}
func (s *state) CloseSession(id string) {
	s.sessions.Delete(id)
	stats.SessionsCount.Set(float64(s.sessions.Count()))
}
