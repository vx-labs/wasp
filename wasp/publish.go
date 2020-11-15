package wasp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vx-labs/commitlog/stream"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/distributed"
	"github.com/vx-labs/wasp/wasp/stats"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

/*
	Message processing follows a simple workflow:

	Distribution -> Scheduling -> Write

	Distribution attempt to resolve Wasp peers hosting a Session subscribed to the message's topic, and write the message in their message log.

	Scheduling process all the messages written in the Log, resolve local recipients, and schedule the distribution by putting the message Log offset in their queue

	Writting process all the message offsets in each session queue, and write them on the wire.
*/

type messageLog interface {
	io.Closer
	Append(b *packet.Publish) error
	Get(offset uint64) (*packet.Publish, error)
	Consume(ctx context.Context, consumerName string, f func(uint64, *packet.Publish) error) error
	Stream(ctx context.Context, consumer stream.Consumer, f func(*packet.Publish) error) error
}

// Scheduler schedules message to local recipients
type Scheduler struct {
	ID     uint64
	writer Writer
	logger *zap.Logger
}

// Schedule distributes the message to local subscribers.
func (pdist *Scheduler) Schedule(ctx context.Context, offset uint64, publish *packet.Publish) error {
	pdist.writer.Schedule(ctx, offset)
	return nil
}

type publishDistributorTransport interface {
	Call(id uint64, f func(*grpc.ClientConn) error) error
}

// PublishDistributor stores publish messaes in local or remote message logs.
type PublishDistributor struct {
	ID        uint64
	Transport publishDistributorTransport
	State     distributed.SubscriptionsState
	Storage   messageLog
	Logger    *zap.Logger
}

// Distribute resolves message destinations, and use them to write message on disk.
func (storer *PublishDistributor) Distribute(ctx context.Context, publish *packet.Publish) error {
	started := time.Now()
	defer stats.PublishDistributionTime.Observe(stats.MilisecondsElapsed(started))
	subscriptions := storer.State.ByPattern(publish.Topic)
	destinations := map[uint64]struct{}{}
	for _, subscription := range subscriptions {
		destinations[subscription.Peer] = struct{}{}
	}

	failed := false
	// Do not interrupt delivery if one destination fails, but return error to client
	for peer := range destinations {
		if peer == storer.ID {
			storer.Storage.Append(publish)
			continue
		}
		if storer.Transport == nil {
			continue
		}
		err := storer.Transport.Call(peer, func(c *grpc.ClientConn) error {
			_, err := api.NewMQTTClient(c).DistributeMessage(ctx, &api.DistributeMessageRequest{Message: publish})
			return err
		})
		if err != nil {
			failed = true
			storer.Logger.Warn("failed to distribute publish", zap.Error(err), zap.String("hex_remote_node_id", fmt.Sprintf("%x", peer)))
		}
	}
	if failed {
		return errors.New("delivery failed")
	}
	return nil
}

func SchedulePublishes(id uint64, writer Writer, messageLog messageLog) func(ctx context.Context) {
	return func(ctx context.Context) {
		publishDistributor := &Scheduler{
			ID:     id,
			writer: writer,
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
