package wasp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vx-labs/mqtt-protocol/decoder"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/transport"
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
	session := newSession(c)
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
	err := session.processConnect(connectPkt)
	if err != nil {
		return err
	}
	if string(connectPkt.Username) != "vx:psk" || string(connectPkt.Password) != os.Getenv("PSK_PASSWORD") {
		return enc.ConnAck(&packet.ConnAck{
			Header:     connectPkt.Header,
			ReturnCode: packet.CONNACK_REFUSED_BAD_USERNAME_OR_PASSWORD,
		})
	}
	state.SaveSession(session.id, session)
	defer state.CloseSession(session.id)
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

	for pkt := range dec.Packet() {
		err = processRequest(ctx, state, ch, &Request{
			packet:    pkt,
			conn:      c,
			sessionID: string(connectPkt.ClientId),
		})
		if err == ErrSessionDisconnected {
			return nil
		}
		c.SetDeadline(
			time.Now().Add(2 * time.Duration(keepAlive) * time.Second),
		)
	}
	err = dec.Err()
	if session.lwt != nil {
		ch <- session.lwt
	}
	return err
}

type session struct {
	id   string
	lwt  *packet.Publish
	conn io.Writer
}

func newSession(conn io.Writer) *session {
	return &session{conn: conn}
}

func (s *session) processConnect(connect *packet.Connect) error {
	s.id = string(connect.ClientId)
	if len(connect.WillTopic) > 0 {
		s.lwt = &packet.Publish{
			Header:  &packet.Header{Retain: connect.WillRetain, Qos: connect.WillQos},
			Topic:   connect.WillTopic,
			Payload: connect.WillPayload,
		}
	}
	return nil
}
func (s *session) Send(publish *packet.Publish) error {
	publish.MessageId = 1
	return encoder.New(s.conn).Publish(publish)
}
