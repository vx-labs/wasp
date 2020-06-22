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
