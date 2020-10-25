package wasp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vx-labs/commitlog/stream"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/sessions"
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

//WriteJob represents an intent to write a packet to a session
type WriteJob struct {
	W       *encoder.Encoder
	Publish *packet.Publish
	Cancel  chan struct{}
	Err     chan error
}

func (job WriteJob) Write(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	retries := 5
	for retries > 0 {
		retries--
		err := job.W.Publish(job.Publish)
		if err != nil {
			job.Err <- err
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !job.Publish.Header.Dup {
				job.Publish.Header.Dup = true
			}
		case <-job.Cancel:
			return
		}
	}
}

// runWriters start a given number of goroutine that will write publish to io.Writer, and wait for an ack.
func runWriters(ctx context.Context, count int) chan chan WriteJob {
	jobs := make(chan chan WriteJob)
	for i := 0; i < count; i++ {
		go func() {
			ch := make(chan WriteJob)
			for {
				select {
				case <-ctx.Done():
					return
				case jobs <- ch:
				case job := <-ch:
					job.Write(ctx)
				}
			}
		}()
	}
	return jobs
}

type writerScheduler struct {
	jobs chan chan WriteJob
}

func (w *writerScheduler) Write(enc *encoder.Encoder, publish *packet.Publish, cancel chan struct{}) error {
	payload := WriteJob{
		W: enc, Publish: publish, Cancel: cancel, Err: make(chan error),
	}
	select {
	case <-cancel:
		return nil
	case runner := <-w.jobs:
		select {
		case runner <- payload:
			break
		case <-cancel:
			return nil
		}
		select {
		case <-cancel:
			return nil
		case err := <-payload.Err:
			return err
		}
	}
}

type WriterScheduler interface {
	Write(w *encoder.Encoder, publish *packet.Publish, cancel chan struct{}) error
}

func NewWriterScheduler(ctx context.Context) WriterScheduler {
	return &writerScheduler{
		jobs: runWriters(ctx, 20),
	}
}
