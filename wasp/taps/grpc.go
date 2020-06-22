package taps

import (
	"context"
	"time"

	"github.com/vx-labs/mqtt-protocol/packet"
	"google.golang.org/grpc"
)

func GRPC(remote *grpc.ClientConn) (Tap, error) {
	client := NewTapClient(remote)
	return func(ctx context.Context, sender string, p *packet.Publish) error {
		_, err := client.PutWaspRecords(ctx, &PutWaspRecordRequest{
			WaspRecords: []*WaspRecord{
				{
					Timestamp: time.Now().UnixNano(),
					Topic:     p.Topic,
					Payload:   p.Payload,
					Retained:  p.Header.Retain,
					Sender:    sender,
				},
			},
		})
		return err
	}, nil
}
