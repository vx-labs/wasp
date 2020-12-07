package wasp

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/v4/wasp/ack"
	"github.com/vx-labs/wasp/v4/wasp/distributed"
	"github.com/vx-labs/wasp/v4/wasp/sessions"
	"github.com/vx-labs/wasp/v4/wasp/stats"
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
	queue     chan RoutedMessage
	state     distributed.SubscriptionsState
	local     LocalState
	inflights ack.Queue
	midPool   *gotomic.List
	encoder   *encoder.Encoder
}

type Writer interface {
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
func (w *writer) sendQoS1(ctx context.Context, publish *packet.Publish, session *sessions.Session) error {
	session.ExtendDeadline()
	stats.EgressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(publish.Length()))

	err := w.inflights.Insert(session.ID(), publish, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
		if expired && w.local.Get(session.ID()) != nil {
			w.sendQoS1(ctx, publish, session)
		} else {
			w.midPool.Push(stored.(*packet.Publish).MessageId)
		}
	})
	if err != nil {
		return err
	}
	// do not return encoder error to avoid freeing message id
	err = w.encoder.Publish(session.Writer(), publish)
	if err != nil {
		L(ctx).Error("failed to write message to session", zap.Int32("qos_level", 1), zap.Error(err), zap.String("session_id", session.ID()))
	}
	return nil
}
func (w *writer) completeQoS2(ctx context.Context, pubRel *packet.PubRel, session *sessions.Session) {
	session.ExtendDeadline()
	stats.EgressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(pubRel.Length()))

	w.inflights.Insert(session.ID(), pubRel, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
		if expired && w.local.Get(session.ID()) != nil {
			w.completeQoS2(ctx, pubRel, session)
			return
		}
		w.midPool.Push(stored.(*packet.PubRel).MessageId)
	})
	err := w.encoder.Encode(session.Writer(), pubRel)
	if err != nil {
		L(ctx).Error("failed to write message to session", zap.Int32("qos_level", 2), zap.Error(err), zap.String("session_id", session.ID()))
	}
}
func (w *writer) sendQoS2(ctx context.Context, publish *packet.Publish, session *sessions.Session) error {

	stats.EgressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(publish.Length()))

	session.ExtendDeadline()

	err := w.inflights.Insert(session.ID(), publish, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
		if w.local.Get(session.ID()) == nil {
			w.midPool.Push(stored.(*packet.Publish).MessageId)
			return
		}
		if expired {
			w.sendQoS2(ctx, publish, session)
			return
		}
		pubRel := &packet.PubRel{
			Header:    &packet.Header{},
			MessageId: stored.(*packet.Publish).MessageId,
		}
		w.completeQoS2(ctx, pubRel, session)
	})
	if err != nil {
		return err
	}
	// do not return encoder error to avoid freeing message id
	err = w.encoder.Publish(session.Writer(), publish)
	if err != nil {
		L(ctx).Error("failed to write message to session", zap.Int32("qos_level", 1), zap.Error(err), zap.String("session_id", session.ID()))
	}
	return nil
}
func (w *writer) getFree(ctx context.Context) (int32, error) {
	retries := 5
	for retries > 0 {
		retries--
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
	return 0, errors.New("failed to get free mid")
}
func (w *writer) send(ctx context.Context, recipients []string, qosses []int32, p *packet.Publish) {
	started := time.Now()
	defer stats.PublishWritingTime.Observe(stats.MilisecondsElapsed(started))
	if p == nil {
		return
	}
	for idx := range recipients {
		sessionID := recipients[idx]
		session := w.local.Get(sessionID)
		if session != nil {
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
			conn := session.Writer()
			switch publish.Header.Qos {
			case 0:
				session.ExtendDeadline()
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
				err = w.sendQoS1(ctx, publish, session)
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
				err = w.sendQoS2(ctx, publish, session)
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
			L(ctx).Debug("message written")
		}
	}
}
