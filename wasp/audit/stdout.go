package audit

import "fmt"

type stdoutRecorder struct {
}

func (s *stdoutRecorder) RecordEvent(tenant string, eventKind event, payload map[string]interface{}) error {
	_, err := fmt.Printf("[%s] %s\n", tenant, eventKind)
	return err
}

func StdoutRecorder() Recorder {
	return &stdoutRecorder{}
}
