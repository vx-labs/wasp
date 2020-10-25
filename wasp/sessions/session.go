package sessions

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/vx-labs/commitlog/stream"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/subscriptions"
)

var (
	ErrInflightQueueFull = errors.New("inflight queue is full")
	ErrUnsupportedQoS    = errors.New("unsupported QoS")
)

const (
	maxInflightSize = 500
)

type Log interface {
	Stream(ctx context.Context, consumer stream.Consumer, f func(*packet.Publish) error) error
}

func getLowerQos(a, b int32) int32 {
	if a > b {
		return b
	}
	return a
}

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
	inflightMtx       sync.RWMutex
	inflight          map[int32]func()
	topics            subscriptions.Tree
	messages          Log
	queueIterator     *queueIterator
}

func NewSession(id, mountpoint string, c io.Writer, messages Log, connect *packet.Connect) (*Session, error) {
	enc := encoder.New(c,
		encoder.WithStatRecorder(stats.GaugeVec("egressBytes").With(map[string]string{
			"protocol": "mqtt",
		})),
	)

	s := &Session{
		ID:            id,
		MountPoint:    mountpoint,
		conn:          c,
		Encoder:       enc,
		inflight:      make(map[int32]func(), 20),
		messages:      messages,
		queueIterator: &queueIterator{},
		topics:        subscriptions.NewTree(),
	}
	return s, s.processConnect(connect)
}
func (s *Session) RunDistributor(ctx context.Context) chan error {
	ch := make(chan error)
	go func() {
		defer close(ch)
		consumer := stream.NewConsumer(
			stream.WithEOFBehaviour(stream.EOFBehaviourPoll),
			stream.WithOffsetIterator(s.queueIterator))

		ch <- s.messages.Stream(ctx, consumer, s.Send)
	}()
	return ch
}
func (s *Session) LWT() *packet.Publish {
	return s.lwt
}

func (s *Session) freeMsgID() int32 {
	var i int32
	for i = 1; i < maxInflightSize; i++ {
		if _, ok := s.inflight[i]; !ok {
			return i
		}
	}
	return 0
}

func (s *Session) PubAck(msgID int32) {
	s.inflightMtx.Lock()
	defer s.inflightMtx.Unlock()
	if _, ok := s.inflight[msgID]; ok {
		s.inflight[msgID]()
		delete(s.inflight, msgID)
	}
}

func (s *Session) sendQos0(publish packet.Publish) error {
	return s.Encoder.Publish(&publish)
}
func (s *Session) sendQos1(publish packet.Publish) error {
	ch := make(chan struct{})
	ticker := time.NewTicker(3 * time.Second)

	s.inflightMtx.Lock()
	publish.MessageId = s.freeMsgID()
	s.inflight[publish.MessageId] = func() {
		close(ch)
	}
	s.inflightMtx.Unlock()

	for {
		err := s.Encoder.Publish(&publish)
		if err != nil {
			return err
		}
		select {
		case <-ticker.C:
			if !publish.Header.Dup {
				publish.Header.Dup = true
			}
		case <-ch:
			return nil
		}
	}
}

func (s *Session) Schedule(id uint64) error {
	s.queueIterator.Push(id)
	return nil
}

// Send write the publish packet into the sesson connection, and wait for an ACK if qos > 0
func (s *Session) Send(publish *packet.Publish) error {
	if len(s.MountPoint) > len(publish.Topic) {
		return nil
	}
	recipients := []string{}
	recipientQos := []int32{}
	recipientPeer := []uint64{}
	err := s.topics.Match(publish.Topic, &recipientPeer, &recipients, &recipientQos)
	if err != nil {
		return err
	}
	qos := publish.Header.Qos
	if len(recipientQos) > 0 {
		qos = recipientQos[0]
	}
	if qos > 1 {
		// TODO: support QoS2
		qos = 1
	}

	outgoing := packet.Publish{
		Header: &packet.Header{
			Dup:    publish.Header.Dup,
			Qos:    qos,
			Retain: publish.Header.Retain,
		},
		Topic:     TrimMountPoint(s.MountPoint, publish.Topic),
		MessageId: 1,
		Payload:   publish.Payload,
	}

	switch outgoing.Header.Qos {
	case 0:
		return s.sendQos0(outgoing)
	case 1:
		return s.sendQos1(outgoing)
	default:
		return ErrUnsupportedQoS
	}
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
