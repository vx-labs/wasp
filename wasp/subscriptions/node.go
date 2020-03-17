package subscriptions

import (
	"errors"
	"sync"

	"github.com/vx-labs/wasp/wasp/format"
)

const (
	// MWC is the multi-level wildcard
	MWC = "#"

	// SWC is the single level wildcard
	SWC = "+"
)

var (
	ErrSubscriptionNotFound = errors.New("Subscription not found")
)

type Tree interface {
	Insert(pattern []byte, qos int32, sub string) error
	Remove(pattern []byte, sub string) error
	Match(topic []byte, qos int32, subs *[]string, qoss *[]int32) error
}

func NewTree() Tree {
	return &tree{
		root: newNode(),
	}
}

type tree struct {
	mtx  sync.RWMutex
	root *node
}

func (t *tree) Insert(pattern []byte, qos int32, sub string) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.insert(format.Topic(pattern), qos, sub)
}
func (t *tree) Remove(pattern []byte, sub string) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.remove(format.Topic(pattern), sub)
}
func (this *tree) Match(topic []byte, qos int32, subs *[]string, qoss *[]int32) error {
	this.mtx.RLock()
	defer this.mtx.RUnlock()

	*subs = (*subs)[0:0]
	*qoss = (*qoss)[0:0]

	return this.root.match(topic, qos, subs, qoss)
}

type node struct {
	recipients []string
	qos        []int32

	children map[string]*node
}

func newNode() *node {
	return &node{
		children: make(map[string]*node),
	}
}

func (n *node) insert(topic format.Topic, qos int32, sub string) error {
	topic, token := topic.Next()

	if token == "" {
		for i := range n.recipients {
			if n.recipients[i] == sub {
				n.qos[i] = qos
				return nil
			}
		}
		n.recipients = append(n.recipients, sub)
		n.qos = append(n.qos, qos)

		return nil
	}
	child, ok := n.children[token]
	if !ok {
		child = newNode()
		n.children[token] = child
	}

	return child.insert(topic, qos, sub)
}

func (n *node) remove(topic format.Topic, sub string) error {
	topic, token := topic.Next()

	if token == "" {
		for i := range n.recipients {
			if n.recipients[i] == sub {
				n.recipients[i] = n.recipients[len(n.recipients)-1]
				n.recipients = n.recipients[:len(n.recipients)-1]
				n.qos[i] = n.qos[len(n.qos)-1]
				n.qos = n.qos[:len(n.qos)-1]
				return nil
			}
		}
		return ErrSubscriptionNotFound
	}
	child, ok := n.children[token]
	if !ok {
		return ErrSubscriptionNotFound
	}

	err := child.remove(topic, sub)
	if err != nil {
		return err
	}
	if len(child.recipients) == 0 && len(child.children) == 0 {
		delete(n.children, token)
	}
	return nil
}

func (this *node) matchQos(qos int32, subs *[]string, qoss *[]int32) {
	for i, sub := range this.recipients {
		// If the published QoS is higher than the subscriber QoS, then we skip the
		// subscriber. Otherwise, add to the list.
		if qos <= this.qos[i] {
			*subs = append(*subs, sub)
			*qoss = append(*qoss, qos)
		}
	}
}
func (this *node) match(topic format.Topic, qos int32, subs *[]string, qoss *[]int32) error {
	topic, token := topic.Next()

	if token == "" {
		this.matchQos(qos, subs, qoss)
		return nil
	}

	for k, n := range this.children {
		// If the key is "#", then these subscribers are added to the result set
		if k == MWC {
			n.matchQos(qos, subs, qoss)
		} else if k == SWC || k == token {
			if err := n.match(topic, qos, subs, qoss); err != nil {
				return err
			}
		}
	}

	return nil
}
