package taps

import (
	"context"
	"time"

	"github.com/vx-labs/wasp/wasp/taps/api"

	"github.com/vx-labs/mqtt-protocol/packet"
	"google.golang.org/grpc"
)

func GRPC(remote *grpc.ClientConn) (Tap, error) {
	client := api.NewTapClient(remote)
	return func(ctx context.Context, p *packet.Publish) error {
		_, err := client.PutRecords(ctx, &api.PutRecordsRequest{
			Records: []*api.Record{
				{
					Timestamp: time.Now().UnixNano(),
					Topic:     p.Topic,
					Payload:   p.Payload,
				},
			},
		})
		return err
	}, nil
}
