package audit

import (
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/dustin/go-humanize"
)

type stdoutRecorder struct {
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
	SessionConnected:    parseTemplate("session {{ .session_id | shorten }} connected"),
	SessionDisonnected:  parseTemplate("session {{ .session_id | shorten }} disconnected"),
	SubscriptionCreated: parseTemplate("session {{ .session_id | shorten }} subscribed to topic \"{{ .pattern }}\""),
	SubscriptionDeleted: parseTemplate("session {{ .session_id | shorten }} unsubscribed to topic \"{{ .pattern }}\""),
	PeerLost:            parseTemplate("wasp peer {{ .peer | shorten }} left the cluster"),
}

func (s *stdoutRecorder) RecordEvent(tenant string, eventKind event, payload map[string]string) error {
	tpl, ok := templates[eventKind]
	if ok {
		return tpl.Execute(os.Stdout, payload)
	}
	return nil
}
func StdoutRecorder() Recorder {
	return &stdoutRecorder{}
}
