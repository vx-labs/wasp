package sessions

import (
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/stats"
)

type Session struct {
	ID                string
	ClientID          string
	MountPoint        string
	lwt               *packet.Publish
	keepaliveInterval int32
	conn              io.Writer
	Encoder           *encoder.Encoder
	Disconnected      bool
	mtx               sync.Mutex
	topics            [][]byte
}

func NewSession(id, mountpoint string, c io.Writer, connect *packet.Connect) (*Session, error) {
	enc := encoder.New(c,
		encoder.WithStatRecorder(stats.GaugeVec("egressBytes").With(map[string]string{
			"protocol": "mqtt",
		})),
	)

	s := &Session{
		ID:         id,
		MountPoint: mountpoint,
		conn:       c,
		Encoder:    enc,
	}
	return s, s.processConnect(connect)
}
func (s *Session) LWT() *packet.Publish {
	return s.lwt
}
func (s *Session) Send(publish *packet.Publish) error {
	if len(s.MountPoint) > len(publish.Topic) {
		return nil
	}

	outgoing := &packet.Publish{
		Header:    publish.Header,
		Topic:     TrimMountPoint(s.MountPoint, publish.Topic),
		MessageId: 1,
		Payload:   publish.Payload,
	}
	return s.Encoder.Publish(outgoing)
}

func (s *Session) processConnect(connect *packet.Connect) error {
	s.ClientID = string(connect.ClientId)
	s.keepaliveInterval = connect.KeepaliveTimer
	if len(connect.WillTopic) > 0 {
		s.lwt = &packet.Publish{
			Header:  &packet.Header{Retain: connect.WillRetain, Qos: connect.WillQos},
			Topic:   PrefixMountPoint(s.MountPoint, connect.WillTopic),
			Payload: connect.WillPayload,
		}
	}
	return nil
}
func (s *Session) AddTopic(t []byte) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.topics = append(s.topics, t)
}
func (s *Session) RemoveTopic(t []byte) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for idx := range s.topics {
		if bytes.Equal(s.topics[idx], t) {
			s.topics[idx] = s.topics[len(s.topics)-1]
			s.topics = s.topics[:len(s.topics)-1]
		}
	}
}
func (s *Session) GetTopics() [][]byte {
	return s.topics
}
func (s *Session) NextDeadline(t time.Time) time.Time {
	return t.Add(2 * time.Duration(s.keepaliveInterval) * time.Second)
}
