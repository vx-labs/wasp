package topics

import (
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/format"
)

func NewTree() Store {
	return &tree{
		root: newNode(),
	}
}

type tree struct {
	mtx  sync.RWMutex
	root *Node
}

func (t *tree) Dump() ([]byte, error) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return proto.Marshal(t.root)
}

func (t *tree) Load(buf []byte) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	root := &Node{}
	err := proto.Unmarshal(buf, root)
	if err != nil {
		return err
	}
	t.root = root
	return nil
}
func (t *tree) Count() int {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.root.count(0)
}
func (t *tree) Insert(msg *packet.Publish) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return t.root.insert(format.Topic(msg.Topic), payload)
}
func (t *tree) Remove(topic []byte) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.remove(format.Topic(topic))
}

func (t *tree) Match(topic []byte, msg *[]*packet.Publish) error {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	out := make([][]byte, 0)
	err := t.root.match(format.Topic(topic), &out)
	if err != nil {
		return err
	}
	for idx := range out {
		publish := &packet.Publish{}
		err := proto.Unmarshal(out[idx], publish)
		if err != nil {
			continue
		}
		*msg = append(*msg, publish)
	}
	return nil
}
