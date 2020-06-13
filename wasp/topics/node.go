package topics

//go:generate protoc -I${GOPATH}/src -I${GOPATH}/src/github.com/vx-labs/wasp/wasp/topics/ --go_out=plugins=grpc:. topics.proto

import (
	"errors"

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

func newNode() *Node {
	return &Node{
		Children: make(map[string]*Node),
	}
}

func (n *Node) insert(topic format.Topic, msg []byte) error {
	topic, token := topic.Next()
	if token == "" {
		n.Buf = msg
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
	if n.Buf != nil && len(n.Buf) > 0 {
		counter++
	}
	for key := range n.Children {
		counter = n.Children[key].count(counter)
	}
	return counter
}
func (n *Node) match(topic format.Topic, msgs *[][]byte) error {
	topic, token := topic.Next()
	if token == "" {
		if n.Buf != nil {
			*msgs = append(*msgs, n.Buf)
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

func (n *Node) allRetained(msgs *[][]byte) {
	if n.Buf != nil {
		*msgs = append(*msgs, n.Buf)
	}

	for _, child := range n.Children {
		child.allRetained(msgs)
	}
}
