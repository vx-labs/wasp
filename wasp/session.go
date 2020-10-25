package wasp

import (
	"context"
	"errors"
	"time"

	"github.com/vx-labs/mqtt-protocol/decoder"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/auth"
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/transport"
	"go.uber.org/zap"
)

var (
	Clock                    = time.Now
	ErrConnectNotDone        = errors.New("CONNECT not done")
	ErrSessionLost           = errors.New("Session lost")
	ErrSessionDisconnected   = errors.New("Session disconnected")
	ErrUnknownPacketReceived = errors.New("Received unknown packet type")
	ErrAuthenticationFailed  = errors.New("Authentication failed")
	ErrProtocolViolation     = errors.New("Protocol violation")
)

type AuthenticationHandler func(ctx context.Context, mqtt auth.ApplicationContext) (id string, mountpoint string, err error)

type PublishHandler func(ctx context.Context, sender string, publish *packet.Publish) error

func doAuth(ctx context.Context, connectPkt *packet.Connect, handler AuthenticationHandler) (string, string, error) {
	return handler(ctx,
		auth.ApplicationContext{
			ClientID: connectPkt.ClientId,
			Username: connectPkt.Username,
			Password: connectPkt.Password,
		})
}

func RunSession(ctx context.Context, peer uint64, fsm FSM, state ReadState, c transport.TimeoutReadWriteCloser, publishHandler PublishHandler, authHandler AuthenticationHandler, messages messageLog) error {
	defer c.Close()
	enc := encoder.New(c)
	dec := decoder.Async(c, decoder.WithStatRecorder(stats.GaugeVec("ingressBytes").With(map[string]string{
		"protocol": "mqtt",
	})))
	defer dec.Cancel()
	c.SetDeadline(
		time.Now().Add(10 * time.Second),
	)
	firstPkt := <-dec.Packet()
	if firstPkt == nil {
		return dec.Err()
	}
	connectPkt, ok := firstPkt.(*packet.Connect)
	if !ok {
		return ErrConnectNotDone
	}

	id, mountPoint, err := doAuth(ctx, connectPkt, authHandler)
	if err != nil {
		L(ctx).Info("authentication failed", zap.Error(err))
		return enc.ConnAck(&packet.ConnAck{
			Header:     connectPkt.Header,
			ReturnCode: packet.CONNACK_REFUSED_BAD_USERNAME_OR_PASSWORD,
		})
	}
	session, err := sessions.NewSession(id, mountPoint, c, messages, connectPkt)
	if err != nil {
		return err
	}

	ctx = AddFields(ctx,
		zap.String("session_id", id),
		zap.String("session_mountpoint", mountPoint),
		zap.Time("connected_at", time.Now()),
		zap.String("session_username", string(connectPkt.Username)),
	)
	L(ctx).Debug("session connected")
	if metadata := state.GetSessionMetadatasByClientID(session.ClientID); metadata != nil {
		err := fsm.DeleteSessionMetadata(ctx, metadata.SessionID, metadata.MountPoint)
		if err != nil {
			return err
		}
		L(ctx).Debug("deleted old session metadata")
	}
	err = fsm.CreateSessionMetadata(ctx, session.ID, session.ClientID, session.LWT(), session.MountPoint)
	if err != nil {
		return err
	}
	L(ctx).Debug("session metadata created")
	state.SaveSession(session.ID, session)
	c.SetDeadline(
		session.NextDeadline(time.Now()),
	)
	defer func() {
		state.CloseSession(session.ID)
		topics := session.GetTopics()
		for idx := range topics {
			fsm.Unsubscribe(ctx, session.ID, topics[idx])
		}
		metadata := state.GetSessionMetadatasByClientID(session.ClientID)
		fsm.DeleteSessionMetadata(ctx, session.ID, session.MountPoint)
		if metadata == nil || metadata.SessionID != session.ID || session.Disconnected {
			// Session has reconnected on another peer.
			return
		}
		if !session.Disconnected {
			L(ctx).Debug("session lost")
			if session.LWT() != nil {
				err := publishHandler(ctx, session.ID, session.LWT())
				if err != nil {
					L(ctx).Warn("failed to publish session LWT", zap.Error(err))
				}
			}
		}
	}()
	err = enc.ConnAck(&packet.ConnAck{
		Header:     connectPkt.Header,
		ReturnCode: packet.CONNACK_CONNECTION_ACCEPTED,
	})
	if err != nil {
		return err
	}
	distributorCh := session.RunDistributor(ctx)
	for {
		select {
		case err := <-distributorCh:
			if err != nil {
				L(ctx).Error("distributor crashed", zap.Error(err))
			}
			return err
		case pkt, ok := <-dec.Packet():
			if !ok {
				return nil
			}
			if pkt.Length() > 10*1000*1000 {
				L(ctx).Warn("received an oversized packet",
					zap.String("packet_type", packet.TypeString(pkt)),
					zap.Int("received_packet_size", pkt.Length()),
				)
				return ErrProtocolViolation
			}
			start := time.Now()
			err = processPacket(ctx, peer, fsm, state, publishHandler, session, pkt)
			stats.HistogramVec("sessionPacketHandling").With(map[string]string{
				"packet_type": packet.TypeString(pkt),
			}).Observe(stats.MilisecondsElapsed(start))

			if err != nil {
				if err == ErrSessionDisconnected {
					L(ctx).Debug("session closed")
					session.Disconnected = true
					return nil
				}
				if err == ErrProtocolViolation {
					L(ctx).Warn("procotol violation")
					return err
				}
				L(ctx).Warn("session packet processing failed", zap.Error(err))
				return err
			}
			c.SetDeadline(
				session.NextDeadline(time.Now()),
			)
		}
	}
}
