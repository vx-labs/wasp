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
	"github.com/vx-labs/wasp/wasp/transport"
	"go.uber.org/zap"
)

var (
	ErrConnectNotDone        = errors.New("CONNECT not done")
	ErrSessionLost           = errors.New("Session lost")
	ErrSessionDisconnected   = errors.New("Session disconnected")
	ErrUnknownPacketReceived = errors.New("Received unknown packet type")
	ErrAuthenticationFailed  = errors.New("Authentication failed")
)

func RunSession(state State, c transport.TimeoutReadWriteCloser, ch chan *packet.Publish) error {
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := sessions.NewSession(c)
	enc := encoder.New(c)
	keepAlive := int32(30)
	dec := decoder.Async(c)
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
	if string(connectPkt.Username) != "vx:psk" || string(connectPkt.Password) != os.Getenv("PSK_PASSWORD") {
		return enc.ConnAck(&packet.ConnAck{
			Header:     connectPkt.Header,
			ReturnCode: packet.CONNACK_REFUSED_BAD_USERNAME_OR_PASSWORD,
		})
	}
	state.SaveSession(session.ID, session)
	defer state.CloseSession(session.ID)
	keepAlive = connectPkt.KeepaliveTimer
	c.SetDeadline(
		time.Now().Add(2 * time.Duration(keepAlive) * time.Second),
	)
	err = enc.ConnAck(&packet.ConnAck{
		Header:     connectPkt.Header,
		ReturnCode: packet.CONNACK_CONNECTION_ACCEPTED,
	})
	if err != nil {
		return err
	}
	defer func() {
		topics := session.GetTopics()
		for idx := range topics {
			state.Unsubscribe(ctx, session.ID, topics[idx])
		}
	}()
	for pkt := range dec.Packet() {
		err = processPacket(ctx, state, ch, session, pkt)
		if err == ErrSessionDisconnected {
			return session.Conn.Close()
		}
		if err != nil {
			L(ctx).Warn("session packet processing failed", zap.Error(err))
		}
		c.SetDeadline(
			time.Now().Add(2 * time.Duration(keepAlive) * time.Second),
		)
	}
	err = dec.Err()
	if session.Lwt != nil {
		ch <- session.Lwt
	}
	return err
}
