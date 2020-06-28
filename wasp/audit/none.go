package audit

import context "context"

type noneRecorder struct {
}

func (s *noneRecorder) Consume(ctx context.Context, consumer func(timestamp int64, tenant, service, eventKind string, payload map[string]string)) error {
	<-ctx.Done()
	return nil
}
func (s *noneRecorder) RecordEvent(tenant string, eventKind event, payload map[string]string) error {
	return nil
}
func NoneRecorder() Recorder {
	return &noneRecorder{}
}
