package topics

//go:generate protoc -I${GOPATH}/src -I${GOPATH}/src/github.com/vx-labs/wasp/wasp/topics/ --go_out=plugins=grpc:. topics.proto

import (
	"errors"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/format"
)

const (
	// MWC is the multi-level wildcard
	MWC = "#"

	// SWC is the single level wildcard
	SWC = "+"
)

var (
	ErrTopicNotFound = errors.New("Topic not found")
)

type Store interface {
	Insert(msg *packet.Publish) error
	Remove(topic []byte) error
	Match(topic []byte, msg *[]*packet.Publish) error
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
func (t *tree) Insert(msg *packet.Publish) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.insert(format.Topic(msg.Topic), msg)
}
func (t *tree) Remove(topic []byte) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.remove(format.Topic(topic))
}

func (t *tree) Match(topic []byte, msg *[]*packet.Publish) error {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.root.match(format.Topic(topic), msg)
}

func newNode() *Node {
	return &Node{
		Children: make(map[string]*Node),
	}
}

func (n *Node) insert(topic format.Topic, msg *packet.Publish) error {
	topic, token := topic.Next()
	var err error
	if token == "" {
		n.Buf, err = proto.Marshal(msg)
		if err != nil {
			return err
		}
		if n.Msg == nil {
			n.Msg = &packet.Publish{}
		}
		if err := proto.Unmarshal(n.Buf, n.Msg); err != nil {
			return err
		}
		return nil
	}

	if n.Children == nil {
		n.Children = make(map[string]*Node)
	}

	child, ok := n.Children[token]
	if !ok {
		child = newNode()
		n.Children[token] = child
	}

	return child.insert(topic, msg)
}

func (n *Node) remove(topic format.Topic) error {
	topic, token := topic.Next()
	if token == "" {
		n.Buf = nil
		n.Msg = nil
		return nil
	}
	if n.Children == nil {
		n.Children = make(map[string]*Node)
	}
	child, ok := n.Children[token]
	if !ok {
		return ErrTopicNotFound
	}
	if err := child.remove(topic); err != nil {
		return err
	}
	if len(child.Children) == 0 {
		delete(n.Children, token)
	}
	return nil
}

func (n *Node) count(counter int) int {
	if n.Msg != nil && len(n.Msg.Payload) > 0 {
		counter++
	}
	for key := range n.Children {
		counter = n.Children[key].count(counter)
	}
	return counter
}
func (n *Node) match(topic format.Topic, msgs *[]*packet.Publish) error {
	topic, token := topic.Next()
	if token == "" {
		if n.Msg != nil {
			*msgs = append(*msgs, n.Msg)
		}
		return nil
	}
	if token == MWC {
		n.allRetained(msgs)
	} else if token == SWC {
		for _, child := range n.Children {
			if err := child.match(topic, msgs); err != nil {
				return err
			}
		}
	} else {
		if n.Children == nil {
			n.Children = make(map[string]*Node)
		}
		if child, ok := n.Children[token]; ok {
			if err := child.match(topic, msgs); err != nil {
				return err
			}
		}
	}

	return nil
}

func (n *Node) allRetained(msgs *[]*packet.Publish) {
	if n.Msg != nil {
		*msgs = append(*msgs, n.Msg)
	}

	for _, child := range n.Children {
		child.allRetained(msgs)
	}
}
