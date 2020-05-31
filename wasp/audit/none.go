package audit

type noneRecorder struct {
}

func (s *noneRecorder) RecordEvent(tenant string, eventKind event, payload map[string]interface{}) error {
	return nil
}
func NoneRecorder() Recorder {
	return &noneRecorder{}
}
