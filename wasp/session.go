package wasp

import (
	"context"
	"errors"
	"fmt"
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

func RunSession(ctx context.Context, peer uint64, fsm FSM, state ReadState, c transport.TimeoutReadWriteCloser, publishHandler PublishHandler, authHandler AuthenticationHandler) error {
	defer c.Close()
	session := sessions.NewSession(c, stats.GaugeVec("egressBytes").With(map[string]string{
		"protocol": "mqtt",
	}))
	enc := encoder.New(c)
	keepAlive := int32(30)
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
		fmt.Println(firstPkt)
		return ErrConnectNotDone
	}
	err := session.ProcessConnect(connectPkt)
	if err != nil {
		return err
	}
	id, mountPoint, err := doAuth(ctx, connectPkt, authHandler)
	session.ID = id
	session.MountPoint = mountPoint
	ctx = AddFields(ctx,
		zap.String("session_id", session.ID),
		zap.Time("connected_at", time.Now()),
		zap.String("session_username", string(connectPkt.Username)),
	)
	if err != nil {
		L(ctx).Info("authentication failed", zap.Error(err))
		return enc.ConnAck(&packet.ConnAck{
			Header:     connectPkt.Header,
			ReturnCode: packet.CONNACK_REFUSED_BAD_USERNAME_OR_PASSWORD,
		})
	}
	if session.Lwt != nil {
		session.Lwt.Topic = sessions.PrefixMountPoint(session.MountPoint, session.Lwt.Topic)
	}
	ctx = AddFields(ctx,
		zap.String("session_mountpoint", session.MountPoint),
	)

	L(ctx).Debug("session connected")
	if metadata := state.GetSessionMetadatasByClientID(session.ClientID); metadata != nil {
		err := fsm.DeleteSessionMetadata(ctx, metadata.SessionID, metadata.MountPoint)
		if err != nil {
			return err
		}
		L(ctx).Debug("deleted old session metadata")
	}
	err = fsm.CreateSessionMetadata(ctx, session.ID, session.ClientID, session.Lwt, session.MountPoint)
	if err != nil {
		return err
	}
	L(ctx).Debug("session metadata created")
	state.SaveSession(session.ID, session)
	defer state.CloseSession(session.ID)
	keepAlive = connectPkt.KeepaliveTimer
	c.SetDeadline(
		time.Now().Add(2 * time.Duration(keepAlive) * time.Second),
	)
	defer func() {
		topics := session.GetTopics()
		for idx := range topics {
			fsm.Unsubscribe(ctx, session.ID, topics[idx])
		}
	}()
	defer func() {
		metadata := state.GetSessionMetadatasByClientID(session.ClientID)
		fsm.DeleteSessionMetadata(ctx, session.ID, session.MountPoint)
		if metadata == nil || metadata.SessionID != session.ID || session.Disconnected {
			// Session has reconnected on another peer.
			return
		}
		err = dec.Err()
		if err != nil {
			L(ctx).Debug("session lost", zap.String("loss_reason", err.Error()))
			if session.Lwt != nil {
				err := publishHandler(ctx, session.ID, session.Lwt)
				if err != nil {
					L(ctx).Error("failed to publish session LWT", zap.Error(err))
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
	for pkt := range dec.Packet() {
		if pkt.Length() > 10*1000*1000 {
			L(ctx).Warn("received an oversized packet",
				zap.String("packet_type", packet.TypeString(pkt)),
				zap.Int("received_packet_size", pkt.Length()),
			)
			return session.Close()
		}
		start := time.Now()
		err = processPacket(ctx, peer, fsm, state, publishHandler, session, pkt)
		stats.HistogramVec("sessionPacketHandling").With(map[string]string{
			"packet_type": packet.TypeString(pkt),
		}).Observe(stats.MilisecondsElapsed(start))

		if err == ErrSessionDisconnected {
			L(ctx).Debug("session closed")
			session.Disconnected = true
			return session.Close()
		}
		if err != nil {
			L(ctx).Warn("session packet processing failed", zap.Error(err))
		}
		c.SetDeadline(
			time.Now().Add(2 * time.Duration(keepAlive) * time.Second),
		)
	}
	return err
}
