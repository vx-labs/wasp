package membership

import (
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/memberlist"

	"go.uber.org/zap"
)

type Gossip struct {
	id     string
	mlist  *memberlist.Memberlist
	logger *zap.Logger
	meta   []byte
}

func (m *Gossip) Members() []*memberlist.Node {
	return m.mlist.Members()
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

func (self *Gossip) MemberCount() int {
	if self.mlist == nil {
		return 1
	}
	return self.mlist.NumMembers()
}

func New(id uint64, port int, advertiseAddr string, advertisePort int, logger *zap.Logger) *Gossip {
	idstr := strconv.Itoa(int(id))
	self := &Gossip{
		id:     idstr,
		meta:   []byte{},
		logger: logger,
	}

	config := memberlist.DefaultLANConfig()
	config.AdvertiseAddr = advertiseAddr
	config.AdvertisePort = advertisePort
	config.BindPort = port
	config.BindAddr = "0.0.0.0"
	config.Name = idstr
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
