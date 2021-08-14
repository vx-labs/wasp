package subscriptions

//go:generate protoc -I${GOPATH}/src -I${GOPATH}/src/github.com/vx-labs/wasp/subscriptions/ --go_out=plugins=grpc:. subscriptions.proto
import (
	"errors"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/vx-labs/wasp/v4/wasp/format"
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

type NodeIterator func([]byte)

type Tree interface {
	Upsert(pattern []byte, f func([]byte) []byte) error
	Walk(topic []byte, iterator NodeIterator)
	Iterate(iterator NodeIterator)
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
func (t *tree) Upsert(pattern []byte, f func([]byte) []byte) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.root.update(format.Topic(pattern), f)
	return nil
}

func (this *tree) Walk(topic []byte, iterator NodeIterator) {
	this.mtx.RLock()
	defer this.mtx.RUnlock()
	this.root.walk(topic, iterator)
}
func (this *tree) Iterate(iterator NodeIterator) {
	this.mtx.RLock()
	defer this.mtx.RUnlock()
	this.root.iterate(iterator)
}

func newNode() *Node {
	return &Node{
		Children: make(map[string]*Node),
	}
}

func (n *Node) update(topic format.Topic, f func([]byte) []byte) {
	topic, token := topic.Next()
	if token == "" {
		n.Data = f(n.Data)
	} else {
		child, ok := n.Children[token]
		if !ok {
			child = newNode()
			n.Children[token] = child
		}
		child.update(topic, f)

		if len(child.Data) == 0 && len(child.Children) == 0 {
			delete(n.Children, token)
		}
	}
}

func (this *Node) iterate(iterator NodeIterator) {
	if len(this.Data) > 0 {
		iterator(this.Data)
	}
	for _, n := range this.Children {
		n.iterate(iterator)
	}
}
func (this *Node) walk(topic format.Topic, iterator NodeIterator) {
	topic, token := topic.Next()
	if token == "" {
		iterator(this.Data)
		return
	}

	for k, n := range this.Children {
		// If the key is "#", then these subscribers are added to the result set
		if k == MWC {
			iterator(n.Data)
		} else if k == SWC || k == token {
			n.walk(topic, iterator)
		}
	}
}
