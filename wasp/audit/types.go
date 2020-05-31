package audit

type event string

const (
	SessionConnected     event = "session_connected"
	SessionDisonnected   event = "session_disconnected"
	SubscriptionCreated  event = "subscription_created"
	SubscriptionDeleted  event = "subscription_deleted"
	RetainMessageStored  event = "retained_message_stored"
	RetainMessageDeleted event = "retained_message_deleted"
	PeerLost             event = "peer_lost"
)

type Recorder interface {
	RecordEvent(tenant string, eventKind event, payload map[string]interface{}) error
}
