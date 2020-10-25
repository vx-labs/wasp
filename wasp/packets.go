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
	SubscribeFrom(ctx context.Context, id string, peer uint64, pattern []byte, qos int32) error
	Unsubscribe(ctx context.Context, id string, pattern []byte) error
	DeleteSessionMetadata(ctx context.Context, id, mountpoint string) error
	CreateSessionMetadata(ctx context.Context, id, clientID string, lwt *packet.Publish, mountpoint string) error
}

func processPacket(ctx context.Context, peer uint64, fsm FSM, state ReadState, publishHander PublishHandler, session *sessions.Session, pkt interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	switch p := pkt.(type) {
	case *packet.Connect:
		return ErrProtocolViolation
	case *packet.Publish:
		p.Topic = sessions.PrefixMountPoint(session.MountPoint, p.Topic)
		err := publishHander(ctx, session.ID, p)
		if err != nil {
			return err
		}
		if p.Header.Qos == 1 {
			return session.Encoder.PubAck(&packet.PubAck{
				Header:    &packet.Header{},
				MessageId: p.MessageId,
			})
		}
	case *packet.Subscribe:
		topics := make([][]byte, len(p.Topic))
		for idx := range p.Topic {
			topics[idx] = sessions.PrefixMountPoint(session.MountPoint, p.Topic[idx])
		}
		for idx := range topics {
			err := fsm.Subscribe(ctx, session.ID, topics[idx], p.Qos[idx])
			if err != nil {
				return err
			}
			session.AddTopic(topics[idx], p.Qos[idx])
		}
		err := session.Encoder.SubAck(&packet.SubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
			Qos:       p.Qos,
		})
		if err != nil {
			return err
		}
		go func() {
			//TODO: remove need for a goroutine ?
			for idx := range topics {
				messages, err := state.RetainedMessages(topics[idx])
				if err != nil {
					return
				}
				for _, message := range messages {
					err = session.Send(message)
					if err != nil {
						return
					}
				}
			}
		}()
	case *packet.Unsubscribe:
		topics := make([][]byte, len(p.Topic))
		for idx := range p.Topic {
			topics[idx] = sessions.PrefixMountPoint(session.MountPoint, p.Topic[idx])
		}
		for idx := range topics {
			err := fsm.Unsubscribe(ctx, session.ID, topics[idx])
			if err != nil {
				return err
			}
			session.RemoveTopic(topics[idx])
		}
		return session.Encoder.UnsubAck(&packet.UnsubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
		})
	case *packet.PubAck:
		session.PubAck(p.MessageId)
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
