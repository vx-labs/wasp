package ack

import (
	"errors"
	"time"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/expiration"
	"github.com/zond/gotomic"
)

/*
	PUBLISH qos1 waits for PUBACK
	PUBLISH qos2 waits for PUBREC
	PUBREC waits for PUBREL
	PUBREL waits for PUBCOMP
*/

var (
	ErrQueueFull            = errors.New("queue is full")
	ErrWrongMID             = errors.New("message id not found")
	ErrDupMID               = errors.New("duplicate message id")
	ErrWrongPacketType      = errors.New("wrong packet type")
	ErrUnexpectedPacketType = errors.New("unexpected packet type")
	ErrInvalidQos           = errors.New("invalid qos")
	ErrPingInflight         = errors.New("ping already in flight")
	ErrConnAckInflight      = errors.New("connack already in flight")
)

type Callback func(expired bool, stored, received packet.Packet)
type Queue interface {
	Insert(pkt packet.Packet, deadline time.Time, callback Callback) error
	Ack(pkt packet.Packet) error
	Expire(now time.Time)
}

type message struct {
	state    byte
	pkt      packet.Packet
	pid      int32
	callback Callback
	deadline time.Time
}
type queue struct {
	msg      *gotomic.Hash
	timeouts expiration.List
}

func NewQueue() Queue {
	return &queue{
		msg:      gotomic.NewHash(),
		timeouts: expiration.NewList(),
	}
}

type key int32

func (k key) HashCode() uint32 {
	return uint32(k)
}
func (self key) Equals(t gotomic.Thing) bool {
	return t.(key) == self
}

type Ackers interface {
	GetMessageId() int32
}

func (q *queue) Ack(pkt packet.Packet) error {
	switch p := pkt.(type) {
	case Ackers:
		v, ok := q.msg.Delete(key(p.GetMessageId()))
		if !ok {
			return ErrWrongMID
		}
		msg := v.(message)
		q.timeouts.Delete(p.GetMessageId(), msg.deadline)
		if msg.state != pkt.Type() {
			msg.callback(true, msg.pkt, pkt)
			return ErrUnexpectedPacketType
		}
		msg.callback(false, msg.pkt, pkt)
		return nil
	default:
		return ErrWrongPacketType
	}
}
func (q *queue) Expire(now time.Time) {
	for _, v := range q.timeouts.Expire(now) {
		token := v.(int32)
		v, ok := q.msg.Delete(key(token))
		if ok {
			msg := v.(message)
			msg.callback(true, msg.pkt, nil)
		}
	}
}
func (q *queue) push(mid int32, deadline time.Time, msg message) error {
	ok := q.msg.PutIfMissing(key(mid), msg)
	if !ok {
		return ErrDupMID
	}
	q.timeouts.Insert(mid, deadline)
	return nil
}
func (q *queue) Insert(pkt packet.Packet, deadline time.Time, callback Callback) error {
	switch p := pkt.(type) {
	case *packet.PubRec:
		mid := p.MessageId
		if mid == 0 {
			return ErrWrongMID
		}
		msg := message{
			state:    packet.PUBREL,
			callback: callback,
			pkt:      pkt,
			pid:      mid,
			deadline: deadline,
		}
		return q.push(mid, deadline, msg)
	case *packet.PubRel:
		mid := p.MessageId
		if mid == 0 {
			return ErrWrongMID
		}
		msg := message{
			state:    packet.PUBCOMP,
			callback: callback,
			pkt:      pkt,
			pid:      mid,
		}
		return q.push(mid, deadline, msg)
	case *packet.Publish:
		mid := p.MessageId
		if mid == 0 {
			return ErrWrongMID
		}
		switch p.Header.Qos {
		case 1:
			msg := message{
				state:    packet.PUBACK,
				callback: callback,
				pkt:      pkt,
				pid:      mid,
			}
			return q.push(mid, deadline, msg)
		case 2:
			msg := message{
				state:    packet.PUBREC,
				callback: callback,
				pkt:      pkt,
				pid:      mid,
			}
			return q.push(mid, deadline, msg)
		default:
			return ErrInvalidQos
		}
	default:
		return ErrWrongPacketType
	}
}
