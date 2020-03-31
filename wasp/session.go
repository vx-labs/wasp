package wasp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/vx-labs/mqtt-protocol/decoder"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
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

func RunSession(ctx context.Context, peer uint64, fsm FSM, state ReadState, c transport.TimeoutReadWriteCloser, ch chan *packet.Publish) error {
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
	ctx = AddFields(ctx,
		zap.String("session_id", session.ID),
		zap.Time("connected_at", time.Now()),
		zap.String("session_username", string(connectPkt.Username)),
	)
	if string(connectPkt.Username) != "vx:psk" || string(connectPkt.Password) != os.Getenv("PSK_PASSWORD") {
		L(ctx).Info("authentication failed")
		return enc.ConnAck(&packet.ConnAck{
			Header:     connectPkt.Header,
			ReturnCode: packet.CONNACK_REFUSED_BAD_USERNAME_OR_PASSWORD,
		})
	}
	//L(ctx).Info("session connected")
	if metadata := state.GetSessionMetadatasByClientID(session.ClientID); metadata != nil {
		err := fsm.DeleteSessionMetadata(ctx, metadata.SessionID)
		if err != nil {
			return err
		}
	}
	err = fsm.CreateSessionMetadata(ctx, session.ID, session.ClientID, session.Lwt)
	if err != nil {
		return err
	}
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
		fsm.DeleteSessionMetadata(ctx, session.ID)
		if metadata == nil || metadata.SessionID != session.ID || session.Disconnected {
			// Session has reconnected on another peer.
			return
		}
		err = dec.Err()
		if err != nil {
			L(ctx).Info("session lost", zap.String("loss_reason", err.Error()))
			if session.Lwt != nil {
				ch <- session.Lwt
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
		start := time.Now()
		err = processPacket(ctx, peer, fsm, state, ch, session, pkt)
		stats.HistogramVec("sessionPacketHandling").With(map[string]string{
			"packet_type": packet.TypeString(pkt),
		}).Observe(stats.MilisecondsElapsed(start))

		if err == ErrSessionDisconnected {
			//	L(ctx).Info("session closed")
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
