package wasp

import (
	"context"
	"time"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/stats"
)

func getLowerQos(a, b int32) int32 {
	if a > b {
		return b
	}
	return a
}

func ProcessPublish(ctx context.Context, id uint64, fsm FSM, state ReadState, p *packet.Publish) error {
	start := time.Now()
	defer func() {
		duration := float64(time.Since(start)) / float64(time.Millisecond)
		stats.Histogram("publishProcessingTime").Observe(duration)
	}()
	if p.Header.Retain {
		if len(p.Payload) == 0 {
			err := fsm.DeleteRetainedMessage(ctx, p.Topic)
			if err != nil {
				return err
			}
		} else {
			err := fsm.RetainedMessage(ctx, p)
			if err != nil {
				return err
			}
		}
		p.Header.Retain = false
	}
	peers, recipients, qoss, err := state.Recipients(p.Topic)
	if err != nil {
		return err
	}
	for idx := range recipients {
		publish := &packet.Publish{
			Header: &packet.Header{
				Dup: p.Header.Dup,
				Qos: getLowerQos(qoss[idx], p.Header.Qos),
			},
			Payload: p.Payload,
			Topic:   p.Topic,
		}
		if peers[idx] == id {
			session := state.GetSession(recipients[idx])
			if session != nil {
				session.Send(publish)
			}
		}
	}
	return nil
}
