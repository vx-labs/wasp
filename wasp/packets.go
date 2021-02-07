package wasp

import (
	"context"
	"io"
	"time"

	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/v4/wasp/ack"
	"github.com/vx-labs/wasp/v4/wasp/distributed"
	"github.com/vx-labs/wasp/v4/wasp/sessions"
	"go.uber.org/zap"
)

//PacketProcessor processes MQTT packet and update state accordingly, or distribute messages
type PacketProcessor interface {
	Run(ctx context.Context)
	Process(ctx context.Context, session *sessions.Session, c io.Writer, pkt packet.Packet) error
}

type packetProcessor struct {
	state     distributed.State
	local     LocalState
	writer    Writer
	handler   PublishHandler
	inflights ack.Queue
	encoder   *encoder.Encoder
	publishes chan publishRequestInput
}

type publishRequestInput struct {
	sender  string
	publish *packet.Publish
	cb      func(publish *packet.Publish)
}

// NewPacketProcessor returns a new packet processor
func NewPacketProcessor(local LocalState, state distributed.State, writer Writer, publishHandler PublishHandler, ackQueue ack.Queue) PacketProcessor {
	return &packetProcessor{
		state:     state,
		encoder:   encoder.New(),
		inflights: ackQueue,
		writer:    writer,
		local:     local,
		handler:   publishHandler,
		publishes: make(chan publishRequestInput, 20),
	}
}
func (processor *packetProcessor) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case in := <-processor.publishes:
			processor.handler(in.sender, in.publish, in.cb)
		}
	}
}
func (processor *packetProcessor) publishHandler(ctx context.Context, sender string, publish *packet.Publish, cb func(publish *packet.Publish)) {
	select {
	case processor.publishes <- publishRequestInput{
		sender:  sender,
		publish: publish,
		cb:      cb,
	}:
	case <-ctx.Done():
	}
}

func (processor *packetProcessor) Process(ctx context.Context, session *sessions.Session, c io.Writer, pkt packet.Packet) error {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	switch p := pkt.(type) {
	case *packet.Connect:
		return ErrProtocolViolation
	case *packet.Publish:
		p.Topic = session.PrefixMountPoint(p.Topic)
		if c == nil {
			// c is nil: we are sending a LWT.
			processor.publishHandler(ctx, session.ID(), p, func(publish *packet.Publish) {})
		}
		switch p.Header.Qos {
		case 0, 1:
			processor.publishHandler(ctx, session.ID(), p, func(publish *packet.Publish) {
				if publish.Header.Qos == 1 {
					processor.encoder.PubAck(c, &packet.PubAck{
						Header:    &packet.Header{},
						MessageId: p.MessageId,
					})
				}
			})
		case 2:
			pubrec := &packet.PubRec{
				Header:    &packet.Header{},
				MessageId: p.MessageId,
			}
			err := processor.inflights.Insert(session.ID(), pubrec, time.Now().Add(3*time.Second), func(expired bool, stored, received packet.Packet) {
				if expired {
					L(ctx).Warn("qos2 flow timed out waiting for PUBREL")
					return
				}
				processor.publishHandler(ctx, session.ID(), p, func(publish *packet.Publish) {
					pubcomp := &packet.PubComp{
						Header:    &packet.Header{},
						MessageId: received.(*packet.PubRel).MessageId,
					}
					processor.encoder.Encode(c, pubcomp)
				})
			})
			if err != nil {
				return err
			}
			return processor.encoder.Encode(c, pubrec)
		}
	case *packet.Subscribe:
		topics := make([][]byte, len(p.Topic))
		for idx := range p.Topic {
			topics[idx] = session.PrefixMountPoint(p.Topic[idx])
		}
		for idx := range topics {
			err := processor.state.Subscriptions().Create(session.ID(), topics[idx], p.Qos[idx])
			if err != nil {
				return err
			}
			session.AddTopic(topics[idx])
		}
		err := processor.encoder.SubAck(c, &packet.SubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
			Qos:       p.Qos,
		})
		if err != nil {
			return err
		}
		for idx := range topics {
			messages, err := processor.state.Topics().Get(topics[idx])
			if err != nil {
				return err
			}
			if len(messages) > 0 {
				for _, message := range messages {
					processor.writer.Send(ctx, []string{session.ID()}, []int32{p.Qos[idx]}, message.Publish)
				}
				L(ctx).Debug("sent retained messages", zap.Int("message_count", len(messages)))
			}
		}
	case *packet.Unsubscribe:
		topics := make([][]byte, len(p.Topic))
		for idx := range p.Topic {
			topics[idx] = session.PrefixMountPoint(p.Topic[idx])
		}
		for idx := range topics {
			err := processor.state.Subscriptions().Delete(session.ID(), topics[idx])
			if err != nil {
				return err
			}
			session.RemoveTopic(topics[idx])
		}
		return processor.encoder.UnsubAck(c, &packet.UnsubAck{
			Header:    p.Header,
			MessageId: p.MessageId,
		})
	case *packet.PubAck:
		err := processor.inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack puback", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.PubRec:
		err := processor.inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack pubrec", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.PubRel:
		err := processor.inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack pubrel", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.PubComp:
		err := processor.inflights.Ack(session.ID(), p)
		if err != nil {
			L(ctx).Error("failed to ack pubcomp", zap.Int32("message_id", p.MessageId), zap.Error(err))
		}
	case *packet.Disconnect:
		return ErrSessionDisconnected
	case *packet.PingReq:
		metadata, err := processor.state.SessionMetadatas().ByClientID(session.ClientID())
		if err != nil || metadata.SessionID != session.ID() {
			// Session has reconnected on another peer.
			return ErrSessionDisconnected
		}
		return processor.encoder.PingResp(c, &packet.PingResp{
			Header: p.Header,
		})
	}
	return nil
}
