package audit

type event string

const (
	SessionConnected     event = "session_connected"
	SessionDisonnected   event = "session_disconnected"
	SessionSubscribed    event = "session_subscribed"
	SessionUnsubscribed  event = "session_unsubscribed"
	RetainMessageStored  event = "retained_message_stored"
	RetainMessageDeleted event = "retained_message_deleted"
	PeerLost             event = "peer_lost"
)

type Recorder interface {
	RecordEvent(tenant string, eventKind event, payload map[string]interface{}) error
}
