package taps

import (
	"context"
	"time"

	"github.com/vx-labs/nest/nest/api"

	"github.com/vx-labs/mqtt-protocol/packet"
	"google.golang.org/grpc"
)

func Nest(remote *grpc.ClientConn) (Tap, error) {
	client := api.NewMessagesClient(remote)
	return func(ctx context.Context, p *packet.Publish) error {
		_, err := client.PutRecords(ctx, &api.PutRecordsRequest{
			Records: []*api.Record{
				&api.Record{
					Timestamp: time.Now().UnixNano(),
					Topic:     p.Topic,
					Payload:   p.Payload,
				},
			},
		})
		return err
	}, nil
}
