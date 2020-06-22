package audit

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/wasp/audit/ audit.proto --go_out=plugins=grpc:.

type event string

const (
	SessionConnected    event = "session_connected"
	SessionDisonnected  event = "session_disconnected"
	SubscriptionCreated event = "subscription_created"
	SubscriptionDeleted event = "subscription_deleted"
	PeerLost            event = "peer_lost"
)

type Recorder interface {
	RecordEvent(tenant string, eventKind event, payload map[string]string) error
}
