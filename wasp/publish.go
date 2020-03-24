package wasp

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/rpc"
	"go.uber.org/zap"
)

func getLowerQos(a, b int32) int32 {
	if a > b {
		return b
	}
	return a
}

func ProcessPublish(ctx context.Context, id uint64, transport *rpc.Transport, fsm FSM, state ReadState, local bool, p *packet.Publish) error {
	if p.Header.Retain {
		if len(p.Payload) == 0 {
			err := fsm.DeleteRetainedMessage(ctx, p.Topic)
			if err != nil {
				return err
			}
		} else {
			err := fsm.RetainedMessage(ctx, p)
			if err != nil {
				return err
			}
		}
		p.Header.Retain = false
	}
	peers, recipients, qoss, err := state.Recipients(p.Topic)
	if err != nil {
		return err
	}
	peersDone := map[uint64]struct{}{}
	for idx := range recipients {
		publish := &packet.Publish{
			Header: &packet.Header{
				Dup: p.Header.Dup,
				Qos: getLowerQos(qoss[idx], p.Header.Qos),
			},
			Payload: p.Payload,
			Topic:   p.Topic,
		}
		if peers[idx] == id {
			session := state.GetSession(recipients[idx])
			if session != nil {
				session.Send(publish)
			}
		} else if local {
			if _, ok := peersDone[peers[idx]]; !ok {
				peersDone[peers[idx]] = struct{}{}
				ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
				err := transport.DistributeMessage(ctx, peers[idx], publish)
				cancel()
				if err != nil {
					L(ctx).Warn("failed to distribute message to remote peer",
						zap.Error(err), zap.String("hex_remote_peer_id", fmt.Sprintf("%x", peers[idx])))
				}
			}
		}
	}
	return nil
}
func StorePublish(messageLog MessageLog, p []*packet.Publish) error {
	buf := make([][]byte, len(p))
	for idx := range buf {
		payload, err := proto.Marshal(p[idx])
		if err != nil {
			return err
		}
		buf[idx] = payload
	}
	err := messageLog.Append(buf)
	if err != nil {
		return err
	}
	return nil
}
