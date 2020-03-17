package topics

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
}

func NewTree() Store {
	return &tree{
		root: newNode(),
	}
}

type tree struct {
	mtx  sync.RWMutex
	root *node
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

type node struct {
	msg *packet.Publish
	buf []byte

	children map[string]*node
}

func newNode() *node {
	return &node{
		children: make(map[string]*node),
	}
}

func (n *node) insert(topic format.Topic, msg *packet.Publish) error {
	topic, token := topic.Next()
	var err error
	if token == "" {
		n.buf, err = proto.Marshal(msg)
		if err != nil {
			return err
		}
		if n.msg == nil {
			n.msg = &packet.Publish{}
		}
		if err := proto.Unmarshal(n.buf, n.msg); err != nil {
			return err
		}
		return nil
	}
	// Add snode if it doesn't already exist
	child, ok := n.children[token]
	if !ok {
		child = newNode()
		n.children[token] = child
	}

	return child.insert(topic, msg)
}

func (n *node) remove(topic format.Topic) error {
	topic, token := topic.Next()
	if token == "" {
		n.buf = nil
		n.msg = nil
		return nil
	}
	child, ok := n.children[token]
	if !ok {
		return ErrTopicNotFound
	}
	if err := child.remove(topic); err != nil {
		return err
	}
	if len(child.children) == 0 {
		delete(n.children, token)
	}
	return nil
}

func (n *node) match(topic format.Topic, msgs *[]*packet.Publish) error {
	topic, token := topic.Next()
	if token == "" {
		if n.msg != nil {
			*msgs = append(*msgs, n.msg)
		}
		return nil
	}
	if token == MWC {
		n.allRetained(msgs)
	} else if token == SWC {
		for _, child := range n.children {
			if err := child.match(topic, msgs); err != nil {
				return err
			}
		}
	} else {
		if child, ok := n.children[token]; ok {
			if err := child.match(topic, msgs); err != nil {
				return err
			}
		}
	}

	return nil
}

func (n *node) allRetained(msgs *[]*packet.Publish) {
	if n.msg != nil {
		*msgs = append(*msgs, n.msg)
	}

	for _, child := range n.children {
		child.allRetained(msgs)
	}
}
