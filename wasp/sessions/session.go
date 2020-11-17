package sessions

import (
	"bytes"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
)

type Session struct {
	id                string
	clientID          string
	mountPoint        string
	lwt               []byte
	keepaliveInterval int32
	Disconnected      bool
	topics            [][]byte
	transport         string
}

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

func NewSession(id, mountpoint, transport string, connect *packet.Connect) (*Session, error) {
	s := &Session{
		id:         id,
		mountPoint: mountpoint,
		transport:  transport,
	}
	return s, s.processConnect(connect)
}

func (s *Session) ID() string {
	return s.id
}
func (s *Session) ClientID() string {
	return s.clientID
}
func (s *Session) Transport() string {
	return s.transport
}
func (s *Session) MountPoint() string {
	return s.mountPoint
}
func (s *Session) PrefixMountPoint(topic []byte) []byte {
	return prefixMountPoint(s.mountPoint, topic)
}
func (s *Session) TrimMountPoint(topic []byte) []byte {
	return trimMountPoint(s.mountPoint, topic)
}
func (s *Session) LWT() *packet.Publish {
	if s.lwt == nil {
		return nil
	}
	p := &packet.Publish{}
	err := proto.Unmarshal(s.lwt, p)
	if err != nil {
		return nil
	}
	return p
}

func (s *Session) processConnect(connect *packet.Connect) error {
	s.clientID = string(connect.ClientId)
	s.keepaliveInterval = connect.KeepaliveTimer
	if len(connect.WillTopic) > 0 {
		buf, err := proto.Marshal(&packet.Publish{
			Header:  &packet.Header{Retain: connect.WillRetain, Qos: connect.WillQos},
			Topic:   prefixMountPoint(s.mountPoint, connect.WillTopic),
			Payload: connect.WillPayload,
		})
		if err != nil {
			return err
		}
		s.lwt = buf
	}
	return nil
}
func (s *Session) AddTopic(t []byte) {
	s.topics = append(s.topics, t)
}
func (s *Session) RemoveTopic(new []byte) {
	for idx, t := range s.topics {
		if bytes.Equal(t, new) {
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
