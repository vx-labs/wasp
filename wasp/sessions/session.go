package sessions

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/subscriptions"
)

type Session struct {
	id                string
	clientID          string
	mountPoint        string
	lwt               []byte
	keepaliveInterval int32
	Disconnected      bool
	topics            subscriptions.Tree
	transport         string
}

func NewSession(id, mountpoint, transport string, connect *packet.Connect) (*Session, error) {
	s := &Session{
		id:         id,
		mountPoint: mountpoint,
		transport:  transport,
		topics:     subscriptions.NewTree(),
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
func (s *Session) AddTopic(t []byte, qos int32) {
	s.topics.Insert(0, t, qos, s.id)
}
func (s *Session) RemoveTopic(t []byte) {
	s.topics.Remove(t, s.id)
}
func (s *Session) GetTopics() [][]byte {
	recipients := []string{}
	recipientQos := []int32{}
	recipientPeer := []uint64{}
	recipientPatterns := [][]byte{}
	err := s.topics.List(&recipientPatterns, &recipientPeer, &recipients, &recipientQos)
	if err != nil {
		return nil
	}
	return recipientPatterns
}
func (s *Session) NextDeadline(t time.Time) time.Time {
	return t.Add(2 * time.Duration(s.keepaliveInterval) * time.Second)
}
