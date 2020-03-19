package sessions

import (
	"io"
	"sync"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
)

type Session struct {
	ID   string
	Lwt  *packet.Publish
	Conn io.WriteCloser
}

func (s *Session) ProcessConnect(connect *packet.Connect) error {
	s.ID = string(connect.ClientId)
	if len(connect.WillTopic) > 0 {
		s.Lwt = &packet.Publish{
			Header:  &packet.Header{Retain: connect.WillRetain, Qos: connect.WillQos},
			Topic:   connect.WillTopic,
			Payload: connect.WillPayload,
		}
	}
	return nil
}
func (s *Session) Send(publish *packet.Publish) error {
	publish.MessageId = 1
	return encoder.New(s.Conn).Publish(publish)
}

func NewSession(c io.WriteCloser) *Session {
	return &Session{
		Conn: c,
	}
}

type Store interface {
	Get(id string) *Session
	Save(id string, session *Session)
	Delete(id string)
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
