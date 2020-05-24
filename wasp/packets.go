package wasp

import (
	"context"
	"time"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/sessions"
)

type FSM interface {
	RetainedMessage(ctx context.Context, publish *packet.Publish) error
	DeleteRetainedMessage(ctx context.Context, topic []byte) error
	Subscribe(ctx context.Context, id string, pattern []byte, qos int32) error
	Unsubscribe(ctx context.Context, id string, pattern []byte) error
	DeleteSessionMetadata(ctx context.Context, id string) error
	CreateSessionMetadata(ctx context.Context, id, clientID string, lwt *packet.Publish, mountpoint string) error
}

func processPacket(ctx context.Context, peer uint64, fsm FSM, state ReadState, publishes chan *packet.Publish, session *sessions.Session, pkt interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	switch p := pkt.(type) {
	case *packet.Connect:
		return session.Close()
	case *packet.Publish:
		p.Topic = sessions.PrefixMountPoint(session.MountPoint, p.Topic)
		select {
		case publishes <- p:
		case <-ctx.Done():
			return ctx.Err()
		}
		if p.Header.Qos == 1 {
			session.Encoder.PubAck(&packet.PubAck{
				Header:    &packet.Header{},
				MessageId: p.MessageId,
			})
		}
	case *packet.Subscribe:
		for idx := range p.Topic {
			p.Topic[idx] = sessions.PrefixMountPoint(session.MountPoint, p.Topic[idx])
			err := fsm.Subscribe(ctx, session.ID, p.Topic[idx], p.Qos[idx])
			if err != nil {
				return err
			}
			session.AddTopic(p.Topic[idx])
		}
		err := session.Encoder.SubAck(&packet.SubAck{
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
				err = session.Encoder.Publish(message)
				if err != nil {
					return err
				}
			}
		}
	case *packet.Unsubscribe:
		for idx := range p.Topic {
			p.Topic[idx] = sessions.PrefixMountPoint(session.MountPoint, p.Topic[idx])
			err := fsm.Unsubscribe(ctx, session.ID, p.Topic[idx])
			if err != nil {
				return err
			}
			session.RemoveTopic(p.Topic[idx])
		}
		return session.Encoder.UnsubAck(&packet.UnsubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
		})
	case *packet.Disconnect:
		return ErrSessionDisconnected
	case *packet.PingReq:
		metadata := state.GetSessionMetadatasByClientID(session.ClientID)
		if metadata == nil || metadata.SessionID != session.ID {
			// Session has reconnected on another peer.
			return ErrSessionDisconnected
		}
		return session.Encoder.PingResp(&packet.PingResp{
			Header: p.Header,
		})
	}
	return nil
}
