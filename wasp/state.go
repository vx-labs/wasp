package wasp

import (
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/subscriptions"
	"github.com/vx-labs/wasp/wasp/topics"
)

type State interface {
	Subscribe(id string, pattern []byte, qos int32) error
	Unsubscribe(id string, pattern []byte) error
	Recipients(topic []byte, qos int32) ([]string, []int32, error)
	GetSession(id string) *sessions.Session
	SaveSession(id string, session *sessions.Session)
	CloseSession(id string)
	RetainMessage(msg *packet.Publish) error
	DeleteRetainedMessage(topic []byte) error
	RetainedMessages(topic []byte) ([]*packet.Publish, error)
	Load([]byte) error
	MarshalBinary() ([]byte, error)
}
type ReadState interface {
	Recipients(topic []byte, qos int32) ([]string, []int32, error)
	GetSession(id string) *sessions.Session
	SaveSession(id string, session *sessions.Session)
	CloseSession(id string)
	RetainedMessages(topic []byte) ([]*packet.Publish, error)
}

type state struct {
	subscriptions subscriptions.Tree
	sessions      sessions.Store
	topics        topics.Store
}

func NewState() State {
	return &state{
		sessions:      sessions.NewStore(),
		subscriptions: subscriptions.NewTree(),
		topics:        topics.NewTree(),
	}
}

func (s *state) Load([]byte) error {
	return nil
}
func (s *state) MarshalBinary() ([]byte, error) {
	return nil, nil
}
func (s *state) Subscribe(id string, pattern []byte, qos int32) error {
	return s.subscriptions.Insert(pattern, qos, id)
}
func (s *state) Unsubscribe(id string, pattern []byte) error {
	return s.subscriptions.Remove(pattern, id)
}
func (s *state) Recipients(topic []byte, qos int32) ([]string, []int32, error) {
	recipients := []string{}
	recipientQos := []int32{}
	return recipients, recipientQos, s.subscriptions.Match(topic, qos, &recipients, &recipientQos)
}

func (s *state) GetSession(id string) *sessions.Session {
	return s.sessions.Get(id)
}
func (s *state) SaveSession(id string, session *sessions.Session) {
	s.sessions.Save(id, session)
}
func (s *state) CloseSession(id string) {
	s.sessions.Delete(id)
}
func (s *state) RetainMessage(msg *packet.Publish) error {
	if len(msg.Payload) > 0 {
		return s.topics.Insert(msg)
	}
	return s.topics.Remove(msg.Topic)
}
func (s *state) DeleteRetainedMessage(topic []byte) error {
	return s.topics.Remove(topic)
}
func (s *state) RetainedMessages(topic []byte) ([]*packet.Publish, error) {
	out := []*packet.Publish{}
	return out, s.topics.Match(topic, &out)
}
