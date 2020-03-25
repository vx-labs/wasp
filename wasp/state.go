package wasp

import (
	"encoding/json"

	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/subscriptions"
	"github.com/vx-labs/wasp/wasp/topics"
)

type State interface {
	Subscribe(peer uint64, id string, pattern []byte, qos int32) error
	Unsubscribe(id string, pattern []byte) error
	Recipients(topic []byte) ([]uint64, []string, []int32, error)
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
	Recipients(topic []byte) ([]uint64, []string, []int32, error)
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

type StateDump struct {
	Subscriptions []byte
	Topics        []byte
}

func (s *state) Load(buf []byte) error {
	dump := StateDump{}
	err := json.Unmarshal(buf, &dump)
	if err != nil {
		return err
	}
	err = s.subscriptions.Load(dump.Subscriptions)
	if err != nil {
		return err
	}
	stats.Gauge("subscriptionsCount").Set(float64(s.subscriptions.Count()))
	err = s.topics.Load(dump.Topics)
	if err != nil {
		return err
	}
	stats.Gauge("retainedMessagesCount").Set(float64(s.topics.Count()))
	return nil
}
func (s *state) MarshalBinary() ([]byte, error) {
	subscriptionsDump, err := s.subscriptions.Dump()
	if err != nil {
		return nil, nil
	}
	topicsDump, err := s.topics.Dump()
	if err != nil {
		return nil, nil
	}
	dump := StateDump{
		Subscriptions: subscriptionsDump,
		Topics:        topicsDump,
	}
	return json.Marshal(dump)
}
func (s *state) Subscribe(peer uint64, id string, pattern []byte, qos int32) error {
	err := s.subscriptions.Insert(peer, pattern, qos, id)
	if err == nil {
		stats.Gauge("subscriptionsCount").Inc()
	}
	return err
}
func (s *state) Unsubscribe(id string, pattern []byte) error {
	err := s.subscriptions.Remove(pattern, id)
	if err == nil {
		stats.Gauge("subscriptionsCount").Dec()
	}
	return err
}
func (s *state) Recipients(topic []byte) ([]uint64, []string, []int32, error) {
	recipients := []string{}
	recipientQos := []int32{}
	recipientPeer := []uint64{}
	return recipientPeer, recipients, recipientQos, s.subscriptions.Match(topic, &recipientPeer, &recipients, &recipientQos)
}

func (s *state) GetSession(id string) *sessions.Session {
	return s.sessions.Get(id)
}
func (s *state) SaveSession(id string, session *sessions.Session) {
	stats.Gauge("sessionsCount").Inc()
	s.sessions.Save(id, session)
}
func (s *state) CloseSession(id string) {
	stats.Gauge("sessionsCount").Dec()
	s.sessions.Delete(id)
}
func (s *state) RetainMessage(msg *packet.Publish) error {
	if len(msg.Payload) > 0 {
		err := s.topics.Insert(msg)
		if err == nil {
			stats.Gauge("retainedMessagesCount").Set(float64(s.topics.Count()))
		}
		return err
	}
	err := s.topics.Remove(msg.Topic)
	if err == nil {
		stats.Gauge("retainedMessagesCount").Set(float64(s.topics.Count()))
	}
	return err
}
func (s *state) DeleteRetainedMessage(topic []byte) error {
	err := s.topics.Remove(topic)
	if err == nil {
		stats.Gauge("retainedMessagesCount").Set(float64(s.topics.Count()))
	}
	return err
}
func (s *state) RetainedMessages(topic []byte) ([]*packet.Publish, error) {
	out := []*packet.Publish{}
	return out, s.topics.Match(topic, &out)
}
