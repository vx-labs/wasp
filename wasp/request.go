package wasp

import (
	"context"
	"io"
	"time"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/sessions"
)

type Request struct {
	sessionID string
	packet    interface{}
	conn      io.WriteCloser
}

func processRequest(ctx context.Context, state State, publishes chan *packet.Publish, session *sessions.Session, pkt interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch p := pkt.(type) {
	case *packet.Connect:
		return session.Conn.Close()
	case *packet.Publish:
		select {
		case publishes <- p:
		case <-ctx.Done():
			return ctx.Err()
		}
		if p.Header.Qos == 1 {
			encoder.New(session.Conn).PubAck(&packet.PubAck{
				Header:    &packet.Header{},
				MessageId: p.MessageId,
			})
		}
	case *packet.Subscribe:
		for idx := range p.Topic {
			err := state.Subscribe(ctx, session.ID, p.Topic[idx], p.Qos[idx])
			if err != nil {
				return err
			}
		}
		err := encoder.New(session.Conn).SubAck(&packet.SubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
			Qos:       p.Qos,
		})
		if err != nil {
			return err
		}
		for idx := range p.Topic {
			messages, err := state.RetainedMessages(p.Topic[idx])
			if err != nil {
				return err
			}
			for _, message := range messages {
				err = encoder.New(session.Conn).Publish(message)
				if err != nil {
					return err
				}
			}
		}
	case *packet.Unsubscribe:
		for idx := range p.Topic {
			err := state.Unsubscribe(ctx, session.ID, p.Topic[idx])
			if err != nil {
				return err
			}
		}
		return encoder.New(session.Conn).UnsubAck(&packet.UnsubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
		})
	case *packet.Disconnect:
		return ErrSessionDisconnected
	case *packet.PingReq:
		return encoder.New(session.Conn).PingResp(&packet.PingResp{
			Header: p.Header,
		})
	}
	return nil
}
