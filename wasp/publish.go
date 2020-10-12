package wasp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/stats"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type MessageLog interface {
	io.Closer
	Append(b *packet.Publish) error
}

func getLowerQos(a, b int32) int32 {
	if a > b {
		return b
	}
	return a
}

type Membership interface {
	Call(id uint64, f func(*grpc.ClientConn) error) error
}

func ProcessPublish(ctx context.Context, id uint64, transport Membership, fsm FSM, state ReadState, local bool, p *packet.Publish) error {
	if p == nil {
		return nil
	}
	start := time.Now()
	defer func() {
		if local {
			stats.Histogram("publishLocalProcessingTime").Observe(stats.MilisecondsElapsed(start))
		} else {
			stats.Histogram("publishRemoteProcessingTime").Observe(stats.MilisecondsElapsed(start))
		}
	}()
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
				err := transport.Call(peers[idx], func(c *grpc.ClientConn) error {
					_, err := api.NewMQTTClient(c).DistributeMessage(ctx, &api.DistributeMessageRequest{Message: publish})
					return err
				})
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

// PublishDistributor distributes message to local recipients
type PublishDistributor struct {
	ID     uint64
	State  ReadState
	logger *zap.Logger
}

// Distribute distributes the message to local subscribers.
func (pdist *PublishDistributor) Distribute(ctx context.Context, publish *packet.Publish) error {
	peers, recipients, qoss, err := pdist.State.Recipients(publish.Topic)
	if err != nil {
		return err
	}
	for idx := range recipients {
		if peers[idx] == pdist.ID {
			publish := &packet.Publish{
				Header: &packet.Header{
					Dup: publish.Header.Dup,
					Qos: getLowerQos(qoss[idx], publish.Header.Qos),
				},
				Payload: publish.Payload,
				Topic:   publish.Topic,
			}
			session := pdist.State.GetSession(recipients[idx])
			if session != nil {
				session.Send(publish)
			}
		}
	}
	return nil
}

// PublishStorer stores publish messaes in local or remote message logs.
type PublishStorer struct {
	ID        uint64
	Transport Membership
	State     ReadState
	Storage   MessageLog
	logger    *zap.Logger
}

// Store resolves message destinations, and use them to write message on disk.
func (storer *PublishStorer) Store(ctx context.Context, publish *packet.Publish) error {
	destinations, err := storer.State.Destinations(publish.Topic)
	if err != nil {
		return err
	}
	failed := false
	// Do not interrupt delivery if one destination fails, but return error to client
	for idx := range destinations {
		if destinations[idx] == storer.ID {
			storer.Storage.Append(publish)
			continue
		}
		if storer.Transport == nil {
			continue
		}
		err = storer.Transport.Call(destinations[idx], func(c *grpc.ClientConn) error {
			_, err := api.NewMQTTClient(c).DistributeMessage(ctx, &api.DistributeMessageRequest{Message: publish})
			return err
		})
		if err != nil {
			failed = true
			storer.logger.Warn("failed to distribute publish", zap.Error(err), zap.String("hex_remote_node_id", fmt.Sprintf("%x", destinations[idx])))
		}
	}
	if failed {
		return errors.New("delivery failed")
	}
	return nil
}
