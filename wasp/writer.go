package wasp

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/v4/wasp/ack"
	"github.com/vx-labs/wasp/v4/wasp/distributed"
	"github.com/vx-labs/wasp/v4/wasp/sessions"
	"github.com/vx-labs/wasp/v4/wasp/stats"
	"github.com/vx-labs/wasp/v4/wasp/transport"
	"github.com/zond/gotomic"
	"go.uber.org/zap"
)

type RoutedMessage struct {
	offset     uint64
	publish    *packet.Publish
	recipients []string
	qosses     []int32
}

type chanIterator chan RoutedMessage

func (t chanIterator) AdvanceTo(target uint64) {
	for {
		select {
		case v := <-t:
			if v.offset == target {
				return
			}
		default:
			return
		}
	}
}
func (t chanIterator) Next() (uint64, error) {
	v := <-t
	return v.offset, nil
}

type writer struct {
	mtx       sync.Mutex
	peerID    uint64
	sessions  map[string]transport.TimeoutReadWriteCloser
	queue     chan RoutedMessage
	state     distributed.SubscriptionsState
	local     LocalState
	inflights ack.Queue
	midPool   *gotomic.List
	encoder   *encoder.Encoder
}

type Writer interface {
	Register(sessionID string, enc transport.TimeoutReadWriteCloser)
	Unregister(sessionID string)
	Run(ctx context.Context, log messageLog) error
	Schedule(ctx context.Context, offset uint64)
	Send(ctx context.Context, recipients []string, qosses []int32, p *packet.Publish)
}

func NewWriter(peerID uint64, subscriptions distributed.SubscriptionsState, local LocalState, ackQueue ack.Queue) *writer {
	midPool := gotomic.NewList()
	var i int32
	for i = 500; i > 0; i-- {
		midPool.Push(i)
	}
	return &writer{
		peerID:    peerID,
		state:     subscriptions,
		local:     local,
		inflights: ackQueue,
		sessions:  make(map[string]transport.TimeoutReadWriteCloser),
		encoder:   encoder.New(),
		queue:     make(chan RoutedMessage, 25),
		midPool:   midPool,
	}
}

func (w *writer) Schedule(ctx context.Context, offset uint64) {
	select {
	case w.queue <- RoutedMessage{
		offset: offset,
	}:
	case <-ctx.Done():
	}
}
func (w *writer) Send(ctx context.Context, recipients []string, qosses []int32, p *packet.Publish) {
	select {
	case w.queue <- RoutedMessage{
		publish:    p,
		qosses:     qosses,
		recipients: recipients,
	}:
	case <-ctx.Done():
	}
}
func (w *writer) Register(sessionID string, enc transport.TimeoutReadWriteCloser) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	w.sessions[sessionID] = enc
}
func (w *writer) Unregister(sessionID string) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	delete(w.sessions, sessionID)
}
func (w *writer) sendQoS1(publish *packet.Publish, session *sessions.Session, conn io.Writer) error {

	stats.EgressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(publish.Length()))

	err := w.inflights.Insert(session.ID(), publish, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
		if expired && w.local.Get(session.ID()) != nil {
			w.sendQoS1(publish, session, conn)
		} else {
			w.midPool.Push(stored.(*packet.Publish).MessageId)
		}
	})
	if err != nil {
		return err
	}
	// do not return encoder error to avoid freeing message id
	w.encoder.Publish(conn, publish)
	return nil
}
func (w *writer) completeQoS2(pubRel *packet.PubRel, session *sessions.Session, conn io.Writer) {
	stats.EgressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(pubRel.Length()))

	w.inflights.Insert(session.ID(), pubRel, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
		if expired && w.local.Get(session.ID()) != nil {
			w.completeQoS2(pubRel, session, conn)
		} else {
			w.midPool.Push(stored.(*packet.PubRel).MessageId)
		}
	})
	w.encoder.Encode(conn, pubRel)
}
func (w *writer) sendQoS2(publish *packet.Publish, session *sessions.Session, conn io.Writer) error {

	stats.EgressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(publish.Length()))

	err := w.inflights.Insert(session.ID(), publish, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
		if expired && w.local.Get(session.ID()) != nil {
			w.sendQoS2(publish, session, conn)
		} else {
			pubRel := &packet.PubRel{
				Header:    &packet.Header{},
				MessageId: stored.(*packet.Publish).MessageId,
			}
			w.completeQoS2(pubRel, session, conn)
		}
	})
	if err != nil {
		return err
	}
	// do not return encoder error to avoid freeing message id
	w.encoder.Publish(conn, publish)
	return nil
}
func (w *writer) getFree(ctx context.Context) (int32, error) {
	for {
		mid, ok := w.midPool.Pop()
		if !ok {
			select {
			case <-time.After(100 * time.Millisecond):
				continue
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		return mid.(int32), nil
	}
}
func (w *writer) send(ctx context.Context, recipients []string, qosses []int32, p *packet.Publish) {
	started := time.Now()
	defer stats.PublishWritingTime.Observe(stats.MilisecondsElapsed(started))
	for idx := range recipients {
		sessionID := recipients[idx]
		conn := w.sessions[sessionID]
		session := w.local.Get(sessionID)

		if conn != nil {
			publish := &packet.Publish{
				Header: &packet.Header{
					Dup:    p.Header.Dup,
					Qos:    qosses[idx],
					Retain: p.Header.Retain,
				},
				MessageId: 0,
				Payload:   p.Payload,
				Topic:     session.TrimMountPoint(p.Topic),
			}

			switch publish.Header.Qos {
			case 0:
				err := w.encoder.Publish(conn, publish)
				if err != nil {
					L(ctx).Error("failed to write message to session", zap.Int32("qos_level", 0), zap.Error(err), zap.String("session_id", sessionID))
				}
			case 1:
				mid, err := w.getFree(ctx)
				if err != nil {
					return
				}
				publish.MessageId = mid
				err = w.sendQoS1(publish, session, conn)
				if err != nil {
					w.midPool.Push(mid)
					L(ctx).Error("failed to write message to session", zap.Int32("qos_level", 1), zap.Error(err), zap.String("session_id", sessionID))
				}
			case 2:
				mid, err := w.getFree(ctx)
				if err != nil {
					return
				}
				publish.MessageId = mid
				err = w.sendQoS2(publish, session, conn)
				if err != nil {
					w.midPool.Push(mid)
					L(ctx).Error("failed to write message to session", zap.Int32("qos_level", 1), zap.Error(err), zap.String("session_id", sessionID))
				}
			}
		}
	}
}
func (w *writer) Run(ctx context.Context, log messageLog) error {
	ticker := time.NewTicker(time.Second)
	defer close(w.queue)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case tick := <-ticker.C:
				w.inflights.Expire(tick)
			case <-ctx.Done():
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case routedMessage := <-w.queue:
			if routedMessage.offset != 0 {
				started := time.Now()
				p, err := log.Get(routedMessage.offset)
				if err != nil {
					L(ctx).Warn("failed to read message from log", zap.Error(err))
					continue
				}
				subscriptions := w.state.ByPattern(p.Topic)
				if err != nil {
					L(ctx).Warn("failed to resolve recipients", zap.Error(err))
					continue
				}
				c := 0
				for _, subscription := range subscriptions {
					if subscription.Peer == w.peerID {
						c++
					}
				}
				recipients := make([]string, c)
				qosses := make([]int32, c)
				idx := 0
				for _, subscription := range subscriptions {
					if subscription.Peer == w.peerID {
						recipients[idx] = subscription.SessionID
						qosses[idx] = subscription.QoS
						idx++
					}
				}
				stats.PublishSchedulingTime.Observe(stats.MilisecondsElapsed(started))
				w.mtx.Lock()
				w.send(ctx, recipients, qosses, p)
				w.mtx.Unlock()
			} else {
				w.mtx.Lock()
				w.send(ctx, routedMessage.recipients, routedMessage.qosses, routedMessage.publish)
				w.mtx.Unlock()
			}
		}
	}
}
