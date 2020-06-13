package wasp

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/subscriptions"
	"github.com/vx-labs/wasp/wasp/topics"
)

type ReadState interface {
	Recipients(topic []byte) ([]uint64, []string, []int32, error)
	ListSubscriptions() ([][]byte, []uint64, []string, []int32, error)
	GetSession(id string) *sessions.Session
	SaveSession(id string, session *sessions.Session)
	CloseSession(id string)
	RetainedMessages(topic []byte) ([]*packet.Publish, error)
	ListSessionMetadatas() []*api.SessionMetadatas
	GetSessionMetadatas(id string) *api.SessionMetadatas
	GetSessionMetadatasByClientID(id string) *api.SessionMetadatas
}
type State interface {
	ReadState
	Subscribe(peer uint64, id string, pattern []byte, qos int32) error
	Unsubscribe(id string, pattern []byte) error
	RemoveSubscriptionsForPeer(peer uint64)
	RemoveSubscriptionsForSession(id string)
	RetainMessage(msg *packet.Publish) error
	DeleteRetainedMessage(topic []byte) error
	Load([]byte) error
	MarshalBinary() ([]byte, error)
	CreateSessionMetadata(id string, peer uint64, clientID string, connectedAt int64, lwt *packet.Publish, mountpoint string) error
	DeleteSessionMetadata(id string, peer uint64) error
	DeleteSessionMetadatasByPeer(peer uint64)
}

type sessionMetadatasStore struct {
	mtx  sync.RWMutex
	data map[string]*api.SessionMetadatas
}

func (s *sessionMetadatasStore) Save(m *api.SessionMetadatas) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.data[m.SessionID] = m
}
func (s *sessionMetadatasStore) ByID(id string) *api.SessionMetadatas {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	md, ok := s.data[id]
	if !ok {
		return nil
	}
	return md
}
func (s *sessionMetadatasStore) ByClientID(id string) *api.SessionMetadatas {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	for _, md := range s.data {
		if md.ClientID == id {
			return md
		}
	}
	return nil
}
func (s *sessionMetadatasStore) All() []*api.SessionMetadatas {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	out := make([]*api.SessionMetadatas, len(s.data))
	idx := 0
	for _, md := range s.data {
		out[idx] = md
		idx++
	}
	return out
}
func (s *sessionMetadatasStore) Delete(id string) *api.SessionMetadatas {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if old, ok := s.data[id]; ok {
		delete(s.data, id)
		return old
	}
	return nil
}

func (s *sessionMetadatasStore) DeletePeer(peerID uint64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for id := range s.data {
		if s.data[id].Peer == peerID {
			delete(s.data, id)
		}
	}
}
func (s *sessionMetadatasStore) Dump() ([]byte, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return json.Marshal(s.data)
}
func (s *sessionMetadatasStore) Load(buf []byte) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return json.Unmarshal(buf, &s.data)
}

type state struct {
	subscriptions     subscriptions.Tree
	sessions          sessions.Store
	topics            topics.Store
	sessionsMetadatas *sessionMetadatasStore
}

func NewState() State {
	return &state{
		sessions:          sessions.NewStore(),
		subscriptions:     subscriptions.NewTree(),
		topics:            topics.NewTree(),
		sessionsMetadatas: &sessionMetadatasStore{data: make(map[string]*api.SessionMetadatas)},
	}
}

type StateDump struct {
	Subscriptions    []byte
	Topics           []byte
	SessionMetadatas []byte
}

func (s *state) Load(buf []byte) error {
	dump := StateDump{}
	err := json.Unmarshal(buf, &dump)
	if err != nil {
		return err
	}
	err = s.sessionsMetadatas.Load(dump.SessionMetadatas)
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
	sessionMetadatasDump, err := s.sessionsMetadatas.Dump()
	if err != nil {
		return nil, nil
	}
	dump := StateDump{
		Subscriptions:    subscriptionsDump,
		Topics:           topicsDump,
		SessionMetadatas: sessionMetadatasDump,
	}
	return json.Marshal(dump)
}
func (s *state) Subscribe(peer uint64, id string, pattern []byte, qos int32) error {
	if s.sessionsMetadatas.ByID(id) == nil {
		return errors.New("session not found")
	}
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
func (s *state) RemoveSubscriptionsForPeer(peer uint64) {
	count := s.subscriptions.RemovePeer(peer)
	if count > 0 {
		stats.Gauge("subscriptionsCount").Sub(float64(count))
	}
}
func (s *state) RemoveSubscriptionsForSession(id string) {
	count := s.subscriptions.RemoveSession(id)
	if count > 0 {
		stats.Gauge("subscriptionsCount").Sub(float64(count))
	}
}
func (s *state) DeleteSessionMetadata(id string, peer uint64) error {
	s.sessionsMetadatas.Delete(id)
	return nil
}
func (s *state) DeleteSessionMetadatasByPeer(peer uint64) {
	s.sessionsMetadatas.DeletePeer(peer)
}
func (s *state) GetSessionMetadatas(id string) *api.SessionMetadatas {
	return s.sessionsMetadatas.ByID(id)
}
func (s *state) GetSessionMetadatasByClientID(id string) *api.SessionMetadatas {
	return s.sessionsMetadatas.ByClientID(id)
}
func (s *state) ListSessionMetadatas() []*api.SessionMetadatas {
	return s.sessionsMetadatas.All()
}
func (s *state) CreateSessionMetadata(id string, peer uint64, clientID string, connectedAt int64, lwt *packet.Publish, mountpoint string) error {
	s.sessionsMetadatas.Save(&api.SessionMetadatas{
		SessionID:   id,
		ClientID:    clientID,
		ConnectedAt: connectedAt,
		LWT:         lwt,
		Peer:        peer,
		MountPoint:  mountpoint,
	})
	return nil
}

func (s *state) Recipients(topic []byte) ([]uint64, []string, []int32, error) {
	recipients := []string{}
	recipientQos := []int32{}
	recipientPeer := []uint64{}
	return recipientPeer, recipients, recipientQos, s.subscriptions.Match(topic, &recipientPeer, &recipients, &recipientQos)
}

func (s *state) ListSubscriptions() ([][]byte, []uint64, []string, []int32, error) {
	recipients := []string{}
	recipientQos := []int32{}
	recipientPeer := []uint64{}
	recipientPatterns := [][]byte{}
	return recipientPatterns, recipientPeer, recipients, recipientQos, s.subscriptions.List(&recipientPatterns, &recipientPeer, &recipients, &recipientQos)
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
		payload, err := proto.Marshal(msg)
		if err != nil {
			return err
		}
		err = s.topics.Insert(msg.Topic, payload)
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
	payloads := [][]byte{}
	err := s.topics.Match(topic, &payloads)
	if err != nil {
		return nil, err
	}
	out := make([]*packet.Publish, len(payloads))
	for idx := range payloads {
		pkt := &packet.Publish{}
		err := proto.Unmarshal(payloads[idx], pkt)
		if err != nil {
			continue
		}
		out[idx] = pkt
	}
	return out, nil
}
