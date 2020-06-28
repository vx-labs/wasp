package audit

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

type grpcRecorder struct {
	ctx    context.Context
	client WaspAuditRecorderClient
}

func (s *grpcRecorder) Consume(ctx context.Context, consumer func(timestamp int64, tenant, service, eventKind string, payload map[string]string)) error {
	fromTimestamp := time.Now().UnixNano()
	ticker := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		stream, err := s.client.GetWaspEvents(ctx, &GetWaspEventsRequest{FromTimestamp: fromTimestamp})
		if err != nil {
			continue
		}
		for {
			msg, err := stream.Recv()
			if err != nil {
				fromTimestamp = time.Now().UnixNano()
				continue
			}
			for _, event := range msg.Events {
				attributes := make(map[string]string, len(event.Attributes))
				for _, attribute := range event.Attributes {
					attributes[attribute.Key] = attribute.Value
				}
				consumer(event.Timestamp, event.Tenant, event.Service, event.Kind, attributes)
			}
		}
	}
}

func (s *grpcRecorder) RecordEvent(tenant string, eventKind event, payload map[string]string) error {
	attributes := make([]*WaspEventAttribute, len(payload))
	idx := 0
	for key, value := range payload {
		attributes[idx] = &WaspEventAttribute{Key: key, Value: value}
		idx++
	}
	_, err := s.client.PutWaspEvents(s.ctx, &PutWaspEventRequest{
		Events: []*WaspEvent{
			{Timestamp: time.Now().UnixNano(), Tenant: tenant, Kind: string(eventKind), Attributes: attributes},
		},
	})
	return err
}
func GRPCRecorder(remote *grpc.ClientConn) Recorder {
	client := NewWaspAuditRecorderClient(remote)
	return &grpcRecorder{client: client, ctx: context.Background()}
}
