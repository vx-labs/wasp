package subscriptions

//go:generate protoc -I${GOPATH}/src -I${GOPATH}/src/github.com/vx-labs/wasp/wasp/subscriptions/ --go_out=plugins=grpc:. subscriptions.proto
import (
	"errors"
	"sync"

	"github.com/gogo/protobuf/proto"
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
	Dump() ([]byte, error)
	Load([]byte) error
}

func NewTree() Tree {
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

func newNode() *Node {
	return &Node{
		Children: make(map[string]*Node),
	}
}

func (n *Node) insert(topic format.Topic, qos int32, sub string) error {
	topic, token := topic.Next()

	if token == "" {
		for i := range n.Recipients {
			if n.Recipients[i] == sub {
				n.Qos[i] = qos
				return nil
			}
		}
		n.Recipients = append(n.Recipients, sub)
		n.Qos = append(n.Qos, qos)

		return nil
	}
	child, ok := n.Children[token]
	if !ok {
		child = newNode()
		n.Children[token] = child
	}

	return child.insert(topic, qos, sub)
}

func (n *Node) remove(topic format.Topic, sub string) error {
	topic, token := topic.Next()

	if token == "" {
		for i := range n.Recipients {
			if n.Recipients[i] == sub {
				n.Recipients[i] = n.Recipients[len(n.Recipients)-1]
				n.Recipients = n.Recipients[:len(n.Recipients)-1]
				n.Qos[i] = n.Qos[len(n.Qos)-1]
				n.Qos = n.Qos[:len(n.Qos)-1]
				return nil
			}
		}
		return ErrSubscriptionNotFound
	}
	child, ok := n.Children[token]
	if !ok {
		return ErrSubscriptionNotFound
	}

	err := child.remove(topic, sub)
	if err != nil {
		return err
	}
	if len(child.Recipients) == 0 && len(child.Children) == 0 {
		delete(n.Children, token)
	}
	return nil
}

func (this *Node) matchQos(qos int32, subs *[]string, qoss *[]int32) {
	for i, sub := range this.Recipients {
		// If the published QoS is higher than the subscriber QoS, then we skip the
		// subscriber. Otherwise, add to the list.
		if qos <= this.Qos[i] {
			*subs = append(*subs, sub)
			*qoss = append(*qoss, qos)
		}
	}
}
func (this *Node) match(topic format.Topic, qos int32, subs *[]string, qoss *[]int32) error {
	topic, token := topic.Next()

	if token == "" {
		this.matchQos(qos, subs, qoss)
		return nil
	}

	for k, n := range this.Children {
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
