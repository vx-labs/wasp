package wasp

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/sessions"
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

type inflightMessage struct {
	cb       func()
	retry    func() error
	deadline time.Time
	retries  int
}
type inflightQueue struct {
	data []*inflightMessage
	mtx  *sync.Cond
}

func (q inflightQueue) NextMID() int {
	q.mtx.L.Lock()
	defer q.mtx.L.Unlock()
	for {
		for i := range q.data {
			if q.data[i] == nil {
				return i + 1
			}
		}
		q.mtx.Wait()
	}
}

func (q inflightQueue) TickExpired(now time.Time) {
	q.mtx.L.Lock()
	defer q.mtx.L.Unlock()
	for i := range q.data {
		if q.data[i] != nil && q.data[i].deadline.Before(now) {
			if q.data[i].retries > 0 {
				q.data[i].retries--
				err := q.data[i].retry()
				if err != nil {
					q.data[i] = nil
				} else {
					q.data[i].deadline = now.Add(3 * time.Second)
				}
			} else {
				q.data[i] = nil
				q.mtx.Broadcast()
			}
		}
	}
}
func (q inflightQueue) Insert(i int, cb func(), retry func() error) error {
	if i >= len(q.data) {
		return errors.New("queue too small")
	}
	q.mtx.L.Lock()
	defer q.mtx.L.Unlock()
	q.data[i-1] = &inflightMessage{
		cb:       cb,
		retry:    retry,
		retries:  5,
		deadline: time.Now().Add(3 * time.Second),
	}
	return nil
}
func (q inflightQueue) Delete(i int) {
	if i >= len(q.data) {
		return
	}
	q.mtx.L.Lock()
	defer q.mtx.L.Unlock()
	q.data[i-1] = nil
	q.mtx.Broadcast()
}

func (q inflightQueue) Trigger(i int) {
	if i >= len(q.data) {
		return
	}
	q.mtx.L.Lock()
	defer q.mtx.L.Unlock()
	if q.data[i-1] != nil {
		q.data[i-1].cb()
		q.data[i-1] = nil
	}
	q.mtx.Broadcast()
}

func (q inflightQueue) Count() int {
	q.mtx.L.Lock()
	defer q.mtx.L.Unlock()
	c := 0
	for _, v := range q.data {
		if v != nil {
			c++
		}
	}
	return c
}

type writer struct {
	mtx       sync.Mutex
	peerID    uint64
	sessions  map[string]*encoder.Encoder
	queue     chan RoutedMessage
	state     schedulerState
	inflights inflightQueue
}

type Writer interface {
	Ack(mid int32)
	Register(sessionID string, enc *encoder.Encoder)
	Unregister(sessionID string)
	Run(ctx context.Context, log messageLog) error
	Schedule(ctx context.Context, offset uint64)
	Send(ctx context.Context, recipients []string, qosses []int32, p *packet.Publish)
}

func NewWriter(peerID uint64, subscriptions schedulerState) *writer {
	mtx := sync.Mutex{}
	return &writer{
		peerID: peerID,
		state:  subscriptions,
		inflights: inflightQueue{
			data: make([]*inflightMessage, 250),
			mtx:  sync.NewCond(&mtx),
		},
		sessions: make(map[string]*encoder.Encoder),
		queue:    make(chan RoutedMessage, 25),
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
func (w *writer) Ack(mid int32) {
	w.inflights.Trigger(int(mid))
}
func (w *writer) Register(sessionID string, enc *encoder.Encoder) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	w.sessions[sessionID] = enc
}
func (w *writer) Unregister(sessionID string) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	delete(w.sessions, sessionID)
}

func (w *writer) send(ctx context.Context, recipients []string, qosses []int32, p *packet.Publish) {
	for idx := range recipients {
		sessionID := recipients[idx]
		session := w.sessions[sessionID]
		metadata := w.state.GetSession(sessionID)

		if session != nil {
			mid := w.inflights.NextMID()
			if mid == 0 {
				L(ctx).Error("failed to get free MID")
				break
			}

			publish := &packet.Publish{
				Header: &packet.Header{
					Dup:    p.Header.Dup,
					Qos:    qosses[idx],
					Retain: p.Header.Retain,
				},
				MessageId: int32(mid),
				Payload:   p.Payload,
				Topic:     sessions.TrimMountPoint(metadata.MountPoint, p.Topic),
			}
			err := session.Publish(publish)
			if err != nil {
				L(ctx).Warn("failed to distribute publish to session", zap.Error(err), zap.String("session_id", sessionID))
			} else {
				if publish.Header.Qos > 0 {
					w.inflights.Insert(mid, func() {}, func() error {
						publish := &packet.Publish{
							Header: &packet.Header{
								Dup:    true,
								Qos:    qosses[idx],
								Retain: p.Header.Retain,
							},
							MessageId: int32(mid),
							Payload:   p.Payload,
							Topic:     sessions.TrimMountPoint(metadata.MountPoint, p.Topic),
						}
						return session.Publish(publish)
					})
				}
			}
		}
	}
}
func (w *writer) Run(ctx context.Context, log messageLog) error {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	defer close(w.queue)
	for {
		select {
		case tick := <-ticker.C:
			w.inflights.TickExpired(tick)
		case <-ctx.Done():
			return nil
		case routedMessage := <-w.queue:

			if routedMessage.offset != 0 {
				p, err := log.Get(routedMessage.offset)
				if err != nil {
					L(ctx).Warn("failed to read message from log", zap.Error(err))
					continue
				}
				recipients, qosses, err := w.state.RecipientsForPeer(w.peerID, p.Topic)
				if err != nil {
					L(ctx).Warn("failed to resolve recipients", zap.Error(err))
					continue
				}
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
