package wasp

import (
	"context"
	"io"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
)

type Request struct {
	ctx       context.Context
	sessionID string
	packet    interface{}
	conn      io.WriteCloser
}

func processRequest(state State, publishes chan *packet.Publish, req *Request) error {
	switch p := req.packet.(type) {
	case *packet.Connect:
		req.conn.Close()
	case *packet.Publish:
		select {
		case publishes <- p:
		case <-req.ctx.Done():
			return req.ctx.Err()
		}
		if p.Header.Qos == 1 {
			encoder.New(req.conn).PubAck(&packet.PubAck{
				Header:    &packet.Header{},
				MessageId: p.MessageId,
			})
		}
	case *packet.Subscribe:
		for idx := range p.Topic {
			err := state.Subscribe(req.ctx, req.sessionID, p.Topic[idx], p.Qos[idx])
			if err != nil {
				return err
			}
		}
		err := encoder.New(req.conn).SubAck(&packet.SubAck{
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
				err = encoder.New(req.conn).Publish(message)
				if err != nil {
					return err
				}
			}
		}
	case *packet.Unsubscribe:
		for idx := range p.Topic {
			err := state.Unsubscribe(req.ctx, req.sessionID, p.Topic[idx])
			if err != nil {
				return err
			}
		}
		return encoder.New(req.conn).UnsubAck(&packet.UnsubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
		})
	case *packet.Disconnect:
		return ErrSessionDisconnected
	case *packet.PingReq:
		return encoder.New(req.conn).PingResp(&packet.PingResp{
			Header: p.Header,
		})
	}
	return nil
}
