package topics

import (
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/wasp/wasp/format"
)

type Store interface {
	Insert(topic []byte, payload []byte) error
	Remove(topic []byte) error
	Match(topic []byte, msg *[][]byte) error
	Dump() ([]byte, error)
	Load([]byte) error
	Count() int
}

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
func (t *tree) Insert(topic []byte, payload []byte) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.insert(format.Topic(topic), payload)
}
func (t *tree) Remove(topic []byte) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.remove(format.Topic(topic))
}

func (t *tree) Match(topic []byte, msg *[][]byte) error {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.root.match(format.Topic(topic), msg)
}
