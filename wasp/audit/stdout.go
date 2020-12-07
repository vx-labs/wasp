package audit

import (
	context "context"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/dustin/go-humanize"
)

type stdoutRecorder struct {
	ch chan stdoutRecorderWrappedEvent
}

var FuncMap = template.FuncMap{
	"humanBytes": func(n int64) string {
		return humanize.Bytes(uint64(n))
	},
	"bytesToString": func(b []byte) string { return string(b) },
	"shorten":       func(s string) string { return s[0:8] },
	"parseDate": func(i int64) string {
		return time.Unix(0, i).Format(time.RFC3339)
	},
	"timeToDuration": func(i int64) string {
		return humanize.Time(time.Unix(i, 0))
	},
}

func parseTemplate(body string) *template.Template {
	tpl, err := template.New("").Funcs(FuncMap).Parse(fmt.Sprintf("%s\n", body))
	if err != nil {
		panic(err)
	}
	return tpl
}

var templates = map[event]*template.Template{
	SessionConnected:    parseTemplate("session {{ .session_id | shorten }} connected with client-id {{ .client_id }}"),
	SessionDisonnected:  parseTemplate("session {{ .session_id | shorten }} disconnected"),
	SubscriptionCreated: parseTemplate("session {{ .session_id | shorten }} subscribed to topic \"{{ .pattern }}\""),
	SubscriptionDeleted: parseTemplate("session {{ .session_id | shorten }} unsubscribed to topic \"{{ .pattern }}\""),
	PeerLost:            parseTemplate("wasp peer {{ .peer | shorten }} left the cluster"),
}

type stdoutRecorderWrappedEvent struct {
	tenant  string
	service string
	kind    string
	payload map[string]string
}

func (s *stdoutRecorder) Consume(ctx context.Context, consumer func(timestamp int64, tenant, service, eventKind string, payload map[string]string)) error {
	s.ch = make(chan stdoutRecorderWrappedEvent)
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-s.ch:
			consumer(time.Now().UnixNano(), ev.tenant, ev.service, ev.kind, ev.payload)
		}
	}
}
func (s *stdoutRecorder) RecordEvent(tenant string, eventKind event, payload map[string]string) error {
	if s.ch != nil {
		s.ch <- stdoutRecorderWrappedEvent{tenant: tenant, service: "wasp", kind: string(eventKind), payload: payload}
	}
	tpl, ok := templates[eventKind]
	if ok {
		return tpl.Execute(os.Stdout, payload)
	}
	return nil
}
func StdoutRecorder() Recorder {
	return &stdoutRecorder{}
}
