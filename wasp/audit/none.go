package audit

type noneRecorder struct {
}

func (s *noneRecorder) RecordEvent(tenant string, eventKind event, payload map[string]string) error {
	return nil
}
func NoneRecorder() Recorder {
	return &noneRecorder{}
}
