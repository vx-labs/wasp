package wasp

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/sessions"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type MessageLog interface {
	io.Closer
	Append(b *packet.Publish) error
	Consume(ctx context.Context, consumerName string, f func(*packet.Publish) error) error
}

func getLowerQos(a, b int32) int32 {
	if a > b {
		return b
	}
	return a
}

type PublishDistributorState interface {
	Recipients(topic []byte) ([]uint64, []string, []int32, error)
	GetSession(id string) *sessions.Session
}

// PublishDistributor distributes message to local recipients
type PublishDistributor struct {
	ID     uint64
	State  PublishDistributorState
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

type PublishStorerState interface {
	Destinations(topic []byte) ([]uint64, error)
}
type PublishStorerTransport interface {
	Call(id uint64, f func(*grpc.ClientConn) error) error
}

// PublishStorer stores publish messaes in local or remote message logs.
type PublishStorer struct {
	ID        uint64
	Transport PublishStorerTransport
	State     PublishStorerState
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

func DistributePublishes(id uint64, state PublishDistributorState, messageLog MessageLog) func(ctx context.Context) {
	return func(ctx context.Context) {
		publishDistributor := &PublishDistributor{
			ID:    id,
			State: state,
		}
		messageLog.Consume(ctx, "publish_distributor", func(p *packet.Publish) error {
			err := publishDistributor.Distribute(ctx, p)
			if err != nil {
				L(ctx).Info("publish distribution failed", zap.Error(err))
			}
			return err
		})
	}
}
