package subscriptions

//go:generate protoc -I${GOPATH}/src -I${GOPATH}/src/github.com/vx-labs/wasp/wasp/subscriptions/ --go_out=plugins=grpc:. subscriptions.proto
import (
	"bytes"
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

type NodeIterator func(peer uint64, sub string, qos int32)

type Tree interface {
	Insert(peer uint64, pattern []byte, qos int32, sub string) error
	Remove(pattern []byte, sub string) error
	Match(topic []byte, peers *[]uint64, subs *[]string, qoss *[]int32) error
	MatchForPeer(peer uint64, topic []byte, subs *[]string, qoss *[]int32) error
	MatchPeers(topic []byte, peers *[]uint64) error
	List(topics *[][]byte, peers *[]uint64, subs *[]string, qoss *[]int32) error
	RemovePeer(peer uint64) int
	RemoveSession(id string) int
	Dump() ([]byte, error)
	Load([]byte) error
	Count() int
	Walk(topic []byte, iterator NodeIterator)
	Iterate(iterator NodeIterator)
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

func (t *tree) RemovePeer(peer uint64) int {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.removePeer(peer, 0)
}
func (t *tree) RemoveSession(id string) int {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.removeSession(id, 0)
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
func (t *tree) Insert(peer uint64, pattern []byte, qos int32, sub string) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.insert(peer, format.Topic(pattern), qos, sub)
}
func (t *tree) Remove(pattern []byte, sub string) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.root.remove(format.Topic(pattern), sub)
}

func (this *tree) Count() int {
	this.mtx.RLock()
	defer this.mtx.RUnlock()
	return this.root.count(0)
}
func (this *tree) MatchPeers(topic []byte, peers *[]uint64) error {
	this.mtx.RLock()
	defer this.mtx.RUnlock()
	*peers = (*peers)[0:0]
	this.root.walk(topic, func(peer uint64, sub string, qos int32) {
		for _, p := range *peers {
			if p == peer {
				return
			}
		}
		*peers = append(*peers, peer)
	})

	return nil
}
func (this *tree) Match(topic []byte, peers *[]uint64, subs *[]string, qoss *[]int32) error {
	this.mtx.RLock()
	defer this.mtx.RUnlock()

	*subs = (*subs)[0:0]
	*qoss = (*qoss)[0:0]
	*peers = (*peers)[0:0]

	this.root.walk(topic, func(peer uint64, sub string, qos int32) {
		*subs = append(*subs, sub)
		*qoss = append(*qoss, qos)
		*peers = append(*peers, peer)
	})
	return nil
}
func (this *tree) MatchForPeer(peer uint64, topic []byte, subs *[]string, qoss *[]int32) error {
	this.mtx.RLock()
	defer this.mtx.RUnlock()

	*subs = (*subs)[0:0]
	*qoss = (*qoss)[0:0]

	this.root.walk(topic, func(peer uint64, sub string, qos int32) {
		if peer == peer {
			*subs = append(*subs, sub)
			*qoss = append(*qoss, qos)
		}
	})
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
func (this *tree) List(topics *[][]byte, peers *[]uint64, subs *[]string, qoss *[]int32) error {
	this.mtx.RLock()
	defer this.mtx.RUnlock()

	*subs = (*subs)[0:0]
	*qoss = (*qoss)[0:0]
	*peers = (*peers)[0:0]
	*topics = (*topics)[0:0]

	return this.root.list(nil, topics, peers, subs, qoss)
}

func newNode() *Node {
	return &Node{
		Children: make(map[string]*Node),
	}
}

func (n *Node) removeSession(id string, counter int) int {
	cut := len(n.Recipients)
	for i := range n.Recipients {
		if n.Recipients[i] == id {
			counter++
			if i < len(n.Recipients)-1 {
				n.Recipients[i] = n.Recipients[len(n.Recipients)-1]
				n.Qos[i] = n.Qos[len(n.Qos)-1]
				n.Peer[i] = n.Peer[len(n.Peer)-1]
			}
			cut = cut - 1
		}
	}
	if cut < len(n.Recipients) {
		n.Recipients = n.Recipients[:cut]
		n.Qos = n.Qos[:cut]
		n.Peer = n.Peer[:cut]

	}
	for token, child := range n.Children {
		counter = n.Children[token].removeSession(id, counter)
		if len(child.Recipients) == 0 && len(child.Children) == 0 {
			delete(n.Children, token)
		}
	}
	return counter
}
func (n *Node) removePeer(peer uint64, counter int) int {
	cut := len(n.Recipients)
	for i := range n.Recipients {
		if n.Peer[i] == peer {
			counter++
			if i < len(n.Recipients)-1 {
				n.Recipients[i] = n.Recipients[len(n.Recipients)-1]
				n.Qos[i] = n.Qos[len(n.Qos)-1]
				n.Peer[i] = n.Peer[len(n.Peer)-1]
			}
			cut = cut - 1
		}
	}
	if cut < len(n.Recipients) {
		n.Recipients = n.Recipients[:cut]
		n.Qos = n.Qos[:cut]
		n.Peer = n.Peer[:cut]

	}
	for token, child := range n.Children {
		counter = n.Children[token].removePeer(peer, counter)
		if len(child.Recipients) == 0 && len(child.Children) == 0 {
			delete(n.Children, token)
		}
	}
	return counter
}

func (n *Node) count(counter int) int {
	c := counter + len(n.Recipients)
	for key := range n.Children {
		c = n.Children[key].count(c)
	}
	return c
}
func (n *Node) insert(peer uint64, topic format.Topic, qos int32, sub string) error {
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
		n.Peer = append(n.Peer, peer)

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

	return child.insert(peer, topic, qos, sub)
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
				n.Peer[i] = n.Peer[len(n.Peer)-1]
				n.Peer = n.Peer[:len(n.Peer)-1]
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

func (this *Node) appendRecipents(peers *[]uint64, subs *[]string, qoss *[]int32) {
	for i, sub := range this.Recipients {
		*subs = append(*subs, sub)
		*qoss = append(*qoss, this.Qos[i])
		*peers = append(*peers, this.Peer[i])
	}
}

func (this *Node) applyIterator(iterator NodeIterator) {
	for idx := range this.Recipients {
		iterator(this.Peer[idx], this.Recipients[idx], this.Qos[idx])
	}
}
func (this *Node) iterate(iterator NodeIterator) {
	for _, n := range this.Children {
		n.applyIterator(iterator)
		n.iterate(iterator)
	}
}
func (this *Node) walk(topic format.Topic, iterator NodeIterator) {
	topic, token := topic.Next()

	if token == "" {
		this.applyIterator(iterator)
		return
	}

	for k, n := range this.Children {
		// If the key is "#", then these subscribers are added to the result set
		if k == MWC {
			n.applyIterator(iterator)
		} else if k == SWC || k == token {
			n.walk(topic, iterator)
		}
	}
}

func (this *Node) list(key []byte, topics *[][]byte, peers *[]uint64, subs *[]string, qoss *[]int32) error {
	this.appendRecipents(peers, subs, qoss)
	for i := 0; i < len(this.Recipients); i++ {
		*topics = append(*topics, key)
	}
	for k, n := range this.Children {
		if len(key) > 0 {
			key := bytes.Join([][]byte{key, []byte(k)}, []byte("/"))
			if err := n.list(key, topics, peers, subs, qoss); err != nil {
				return err
			}
		} else {
			if err := n.list([]byte(k), topics, peers, subs, qoss); err != nil {
				return err
			}
		}

	}

	return nil
}
