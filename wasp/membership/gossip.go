package membership

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/hashicorp/memberlist"

	"go.uber.org/zap"
)

type Gossip struct {
	id           string
	mlist        *memberlist.Memberlist
	logger       *zap.Logger
	meta         []byte
	onNodeJoin   func(id string, meta []byte)
	onNodeLeave  func(id string, meta []byte)
	onNodeUpdate func(id string, meta []byte)
}

func (m *Gossip) Members() []*memberlist.Node {
	return m.mlist.Members()
}

func (m *Gossip) OnNodeJoin(f func(id string, meta []byte)) {
	m.onNodeJoin = f
	members := m.mlist.Members()
	for idx := range members {
		n := members[idx]
		if n.Meta != nil {
			f(n.Name, n.Meta)
		}
	}
}
func (m *Gossip) OnNodeLeave(f func(id string, meta []byte)) {
	m.onNodeLeave = f
}
func (m *Gossip) OnNodeUpdate(f func(id string, meta []byte)) {
	m.onNodeUpdate = f
}

func (s *Gossip) Health() string {
	if s.mlist.NumMembers() == 1 {
		return "warning"
	}
	return "ok"
}
func (s *Gossip) NodeMeta(limit int) []byte {
	return s.meta
}

func (m *Gossip) Join(hosts []string) error {
	if len(hosts) == 0 {
		return nil
	}
	_, err := m.mlist.Join(hosts)
	return err
}
func (self *Gossip) UpdateMetadata(meta []byte) {
	self.meta = meta
	self.mlist.UpdateNode(5 * time.Second)
}
func (self *Gossip) Shutdown() error {
	err := self.mlist.Leave(5 * time.Second)
	if err != nil {
		return err
	}
	return self.mlist.Shutdown()
}
func (self *Gossip) isNodeKnown(id string) bool {
	members := self.mlist.Members()
	for _, member := range members {
		if member.Name == id {
			return true
		}
	}
	return false
}
func (self *Gossip) numMembers() int {
	if self.mlist == nil {
		return 1
	}
	return self.mlist.NumMembers()
}
func (self *Gossip) MemberCount() int {
	return self.numMembers()
}

func New(id string, port int, advertiseAddr string, advertisePort int, logger *zap.Logger) *Gossip {
	self := &Gossip{
		id:     id,
		meta:   []byte{},
		logger: logger,
	}

	config := memberlist.DefaultLANConfig()
	config.AdvertiseAddr = advertiseAddr
	config.AdvertisePort = advertisePort
	config.BindPort = port
	config.BindAddr = "0.0.0.0"
	config.Name = id
	config.Delegate = self
	config.Events = self
	if os.Getenv("ENABLE_MEMBERLIST_LOG") != "true" {
		config.LogOutput = ioutil.Discard
	}
	list, err := memberlist.Create(config)
	if err != nil {
		panic(err)
	}
	self.mlist = list
	return self
}
