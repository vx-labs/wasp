package distributed

import (
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/crdt"
	"github.com/vx-labs/wasp/topics"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/stats"
)

type topicsState struct {
	mu    sync.RWMutex
	tree  topics.Store
	bcast *memberlist.TransmitLimitedQueue
}

func newTopicState(bcast *memberlist.TransmitLimitedQueue) *topicsState {
	return &topicsState{
		tree:  topics.NewTree(),
		bcast: bcast,
	}
}
func (t *topicsState) mergeMessages(messages []*api.RetainedMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, msg := range messages {
		if msg.Publish == nil || len(msg.Publish.Topic) == 0 {
			return ErrInvalidPayload
		}

		localList, err := t.get(msg.Publish.Topic)
		if err != nil {
			return err
		}
		if len(localList) > 1 {
			return ErrInvalidPayload
		}
		outdated := true
		if len(localList) == 1 {
			outdated = crdt.IsEntryOutdated(localList[0], msg)
		}
		if outdated {
			if crdt.IsEntryAdded(msg) {
				err = t.set(msg.Publish.Topic, msg)
				if err != nil {
					return err
				}
			} else if crdt.IsEntryRemoved(msg) {
				err = t.set(msg.Publish.Topic, msg)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *topicsState) dump(event *api.StateBroadcastEvent) {
	t.tree.Iterate(func(b []byte) {
		message := &api.RetainedMessage{}
		if proto.Unmarshal(b, message) == nil {
			event.RetainedMessages = append(event.RetainedMessages, message)
		}
	})
}

func (t *topicsState) Set(message *packet.Publish) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	msg := &api.RetainedMessage{
		Publish:   message,
		LastAdded: clock(),
	}
	err := t.set(message.Topic, msg)
	if err != nil {
		return err
	}
	buf, err := proto.Marshal(&api.StateBroadcastEvent{
		RetainedMessages: []*api.RetainedMessage{msg},
	})
	if err != nil {
		return err
	}
	t.bcast.QueueBroadcast(simpleBroadcast(buf))
	return nil
}
func (t *topicsState) set(topic []byte, msg *api.RetainedMessage) error {
	buf, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	old, err := t.tree.Insert(topic, buf)
	if old && crdt.IsEntryRemoved(msg) {
		stats.RetainedMessagesCount.Dec()
	} else if !old && crdt.IsEntryAdded(msg) {
		stats.RetainedMessagesCount.Inc()
	}
	return nil
}

func (t *topicsState) Delete(topic []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	msg := &api.RetainedMessage{
		Publish: &packet.Publish{
			Header:  &packet.Header{},
			Topic:   topic,
			Payload: nil,
		},
		LastDeleted: clock(),
	}
	err := t.set(topic, msg)
	if err != nil {
		return err
	}
	buf, err := proto.Marshal(&api.StateBroadcastEvent{
		RetainedMessages: []*api.RetainedMessage{msg},
	})
	if err != nil {
		return err
	}
	t.bcast.QueueBroadcast(simpleBroadcast(buf))
	return nil
}

func (t *topicsState) Get(pattern []byte) ([]api.RetainedMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	v, err := t.get(pattern)
	if err != nil {
		return nil, err
	}
	c := 0
	for idx := range v {
		if crdt.IsEntryAdded(v[idx]) {
			c++
		}
	}
	out := make([]api.RetainedMessage, c)

	idx := 0
	for _, msg := range v {
		if crdt.IsEntryAdded(v[idx]) {
			out[idx] = *msg
			idx++
		}
	}
	return out, nil
}
func (t *topicsState) get(pattern []byte) ([]*api.RetainedMessage, error) {
	outBuf := make([][]byte, 0)
	err := t.tree.Match(pattern, &outBuf)
	if err != nil {
		return nil, err
	}
	out := make([]*api.RetainedMessage, len(outBuf))
	for idx := range outBuf {
		out[idx] = &api.RetainedMessage{}
		err = proto.Unmarshal(outBuf[idx], out[idx])
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
