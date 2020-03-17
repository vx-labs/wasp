package sessions

import (
	"sync"

	"github.com/vx-labs/mqtt-protocol/packet"
)

type Session interface {
	Send(publish *packet.Publish) error
}

type Store interface {
	Get(id string) Session
	Save(id string, session Session)
	Delete(id string)
}

func NewStore() Store {
	return &store{
		data: map[string]Session{},
	}
}

type store struct {
	data map[string]Session
	mtx  sync.RWMutex
}

func (s *store) Get(id string) Session {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	session, ok := s.data[id]
	if !ok {
		return nil
	}
	return session
}
func (s *store) Save(id string, session Session) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.data[id] = session
}
func (s *store) Delete(id string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	delete(s.data, id)
}
