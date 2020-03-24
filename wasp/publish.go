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
	peers, recipients, qoss, err := state.Recipients(p.Topic, p.Header.Qos)
	if err != nil {
		return err
	}
	peersDone := map[uint64]struct{}{}
	for idx := range recipients {
		if peers[idx] == id {
			session := state.GetSession(recipients[idx])
			if session != nil {
				p.Header.Qos = qoss[idx]
				session.Send(p)
			}
		} else if local {
			if _, ok := peersDone[peers[idx]]; !ok {
				peersDone[peers[idx]] = struct{}{}
				ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
				err := transport.DistributeMessage(ctx, peers[idx], p)
				cancel()
				if err != nil {
					L(ctx).Warn("failed to distribute message to remote peer",
						zap.Error(err), zap.String("remote_peer_id", fmt.Sprintf("%x", peers[idx])))
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
