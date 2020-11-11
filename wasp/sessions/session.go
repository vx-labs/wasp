package sessions

import (
	"context"
	"io"
	"time"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/subscriptions"
)

type Session struct {
	ID                string
	ClientID          string
	MountPoint        string
	lwt               *packet.Publish
	keepaliveInterval int32
	conn              io.Writer
	Disconnected      bool
	topics            subscriptions.Tree
	transport         string
}

func NewSession(ctx context.Context, id, mountpoint, transport string, c io.Writer, connect *packet.Connect) (*Session, error) {
	s := &Session{
		ID:         id,
		MountPoint: mountpoint,
		transport:  transport,
		conn:       c,
		topics:     subscriptions.NewTree(),
	}
	return s, s.processConnect(connect)
}

func (s *Session) Transport() string {
	return s.transport
}
func (s *Session) LWT() *packet.Publish {
	return s.lwt
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
func (s *Session) AddTopic(t []byte, qos int32) {
	s.topics.Insert(0, t, qos, s.ID)
}
func (s *Session) RemoveTopic(t []byte) {
	s.topics.Remove(t, s.ID)
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
