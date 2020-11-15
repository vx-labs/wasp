package sessions

import (
	"sync"
)

func prefixMountPoint(mountPoint string, t []byte) []byte {
	out := make([]byte, len(t)+len(mountPoint)+1)
	copy(out[:len(mountPoint)], []byte(mountPoint))
	out[len(mountPoint)] = '/'
	copy(out[len(mountPoint)+1:], t)
	return out
}
func trimMountPoint(mountPoint string, t []byte) []byte {
	return t[len(mountPoint)+1:] // Trim mountpoint + /
}

type Store interface {
	Get(id string) *Session
	All() []*Session
	Save(id string, session *Session)
	Delete(id string)
	Count() int
}

func NewStore() Store {
	return &store{
		data: map[string]*Session{},
	}
}

type store struct {
	data map[string]*Session
	mtx  sync.RWMutex
}

func (s *store) Count() int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return len(s.data)
}

func (s *store) Get(id string) *Session {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	session, ok := s.data[id]
	if !ok {
		return nil
	}
	return session
}
func (s *store) Save(id string, session *Session) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.data[id] = session
}
func (s *store) Delete(id string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	delete(s.data, id)
}
func (s *store) All() []*Session {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	out := make([]*Session, len(s.data))
	idx := 0
	for _, s := range s.data {
		out[idx] = s
		idx++
	}
	return out
}
