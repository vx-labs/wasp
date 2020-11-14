package ack

import (
	"errors"
	"fmt"
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
	Insert(prefix string, pkt packet.Packet, deadline time.Time, callback Callback) error
	Ack(prefix string, pkt packet.Packet) error
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

type Ackers interface {
	GetMessageId() int32
}

func hashKey(prefix string, id int32) gotomic.Hashable {
	return gotomic.StringKey(fmt.Sprintf("%s/%d", prefix, id))
}

func (q *queue) Ack(prefix string, pkt packet.Packet) error {
	switch p := pkt.(type) {
	case Ackers:
		k := hashKey(prefix, p.GetMessageId())
		v, ok := q.msg.Delete(k)
		if !ok {
			return ErrWrongMID
		}
		msg := v.(message)
		q.timeouts.Delete(k, msg.deadline)
		if msg.state != pkt.Type() {
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
		key := v.(gotomic.StringKey)
		m, ok := q.msg.Delete(key)
		if ok {
			msg := m.(message)
			msg.callback(true, msg.pkt, nil)
		}
	}
}
func (q *queue) push(k gotomic.Hashable, msg message) error {
	if !q.msg.PutIfMissing(k, msg) {
		return ErrDupMID
	}
	q.timeouts.Insert(k, msg.deadline)
	return nil
}
func (q *queue) Insert(prefix string, pkt packet.Packet, deadline time.Time, callback Callback) error {
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
		return q.push(hashKey(prefix, mid), msg)
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
			deadline: deadline,
		}
		return q.push(hashKey(prefix, mid), msg)
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
				deadline: deadline,
			}
			return q.push(hashKey(prefix, mid), msg)
		case 2:
			msg := message{
				state:    packet.PUBREC,
				callback: callback,
				pkt:      pkt,
				pid:      mid,
				deadline: deadline,
			}
			return q.push(hashKey(prefix, mid), msg)
		default:
			return ErrInvalidQos
		}
	default:
		return ErrWrongPacketType
	}
}
