package wasp

import (
	"context"
	"io"
	"time"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/ack"
	"github.com/vx-labs/wasp/wasp/sessions"
	"go.uber.org/zap"
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

func processPacket(ctx context.Context, fsm FSM, state ReadState, publishHander PublishHandler, writer Writer, inflights ack.Queue, session *sessions.Session, encoder *encoder.Encoder, c io.Writer, pkt interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	switch p := pkt.(type) {
	case *packet.Connect:
		return ErrProtocolViolation
	case *packet.Publish:
		p.Topic = sessions.PrefixMountPoint(session.MountPoint, p.Topic)
		switch p.Header.Qos {
		case 0, 1:
			err := publishHander(ctx, session.ID(), p)
			if err != nil {
				return err
			}
		case 2:
			pubrec := &packet.PubRec{
				Header:    &packet.Header{},
				MessageId: p.MessageId,
			}
			err := inflights.Insert(session.ID(), pubrec, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
				if expired {
					L(ctx).Warn("qos2 flow timed out at waiting for PUBREL")
					return
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				err := publishHander(ctx, session.ID(), p)
				if err == nil {
					pubcomp := &packet.PubComp{
						Header:    &packet.Header{},
						MessageId: received.(*packet.PubRel).MessageId,
					}
					encoder.Encode(c, pubcomp)
					return
				}
				L(ctx).Error("failed to finalize qos2 flow", zap.Error(err))
			})
			if err != nil {
				return err
			}
			return encoder.Encode(c, pubrec)
		}
		if p.Header.Qos == 1 {
			return encoder.PubAck(c, &packet.PubAck{
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
			err := fsm.Subscribe(ctx, session.ID(), topics[idx], p.Qos[idx])
			if err != nil {
				return err
			}
			session.AddTopic(topics[idx], p.Qos[idx])
		}
		err := encoder.SubAck(c, &packet.SubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
			Qos:       p.Qos,
		})
		if err != nil {
			return err
		}
		for idx := range topics {
			messages, err := state.RetainedMessages(topics[idx])
			if err != nil {
				return err
			}
			for _, message := range messages {
				writer.Send(ctx, []string{session.ID()}, []int32{p.Qos[idx]}, message)
			}
		}
	case *packet.Unsubscribe:
		topics := make([][]byte, len(p.Topic))
		for idx := range p.Topic {
			topics[idx] = sessions.PrefixMountPoint(session.MountPoint, p.Topic[idx])
		}
		for idx := range topics {
			err := fsm.Unsubscribe(ctx, session.ID(), topics[idx])
			if err != nil {
				return err
			}
			session.RemoveTopic(topics[idx])
		}
		return encoder.UnsubAck(c, &packet.UnsubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
		})
	case *packet.PubAck:
		err := inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack puback", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.PubRec:
		err := inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack pubrec", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.PubRel:
		err := inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack pubrel", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.PubComp:
		err := inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack pubcomp", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.Disconnect:
		return ErrSessionDisconnected
	case *packet.PingReq:
		metadata := state.GetSessionMetadatasByClientID(session.ClientID)
		if metadata == nil || metadata.SessionID != session.ID() {
			// Session has reconnected on another peer.
			return ErrSessionDisconnected
		}
		return encoder.PingResp(c, &packet.PingResp{
			Header: p.Header,
		})
	}
	return nil
}
