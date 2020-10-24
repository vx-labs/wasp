package wasp

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/vx-labs/commitlog/stream"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/sessions"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type messageLog interface {
	io.Closer
	Append(b *packet.Publish) error
	Consume(ctx context.Context, consumerName string, f func(uint64, *packet.Publish) error) error
	Stream(ctx context.Context, consumer stream.Consumer, f func(*packet.Publish) error) error
}

type schedulerState interface {
	Recipients(topic []byte) ([]uint64, []string, []int32, error)
	GetSession(id string) *sessions.Session
}

// Scheduler schedules message to local recipients
type Scheduler struct {
	ID     uint64
	State  schedulerState
	logger *zap.Logger
}

// Schedule distributes the message to local subscribers.
func (pdist *Scheduler) Schedule(ctx context.Context, offset uint64, publish *packet.Publish) error {
	peers, recipients, _, err := pdist.State.Recipients(publish.Topic)
	if err != nil {
		return err
	}
	for idx := range recipients {
		if peers[idx] == pdist.ID {
			session := pdist.State.GetSession(recipients[idx])
			if session != nil {
				err := session.Schedule(offset)
				if err != nil {
					L(ctx).Warn("failed to distribute publish to session", zap.Error(err), zap.String("session_id", session.ID))
				}
			}
		}
	}
	return nil
}

type publishDistributorState interface {
	Destinations(topic []byte) ([]uint64, error)
}
type publishDistributorTransport interface {
	Call(id uint64, f func(*grpc.ClientConn) error) error
}

// PublishDistributor stores publish messaes in local or remote message logs.
type PublishDistributor struct {
	ID        uint64
	Transport publishDistributorTransport
	State     publishDistributorState
	Storage   messageLog
	logger    *zap.Logger
}

// Distribute resolves message destinations, and use them to write message on disk.
func (storer *PublishDistributor) Distribute(ctx context.Context, publish *packet.Publish) error {
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

func SchedulePublishes(id uint64, state schedulerState, messageLog messageLog) func(ctx context.Context) {
	return func(ctx context.Context) {
		publishDistributor := &Scheduler{
			ID:    id,
			State: state,
		}
		messageLog.Consume(ctx, "publish_distributor", func(offset uint64, p *packet.Publish) error {
			err := publishDistributor.Schedule(ctx, offset, p)
			if err != nil {
				L(ctx).Info("publish distribution failed", zap.Error(err))
			}
			return err
		})
	}
}
