package membership

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/vx-labs/wasp/wasp/api"
	"github.com/vx-labs/wasp/wasp/rpc"
	"google.golang.org/grpc"

	"go.uber.org/zap"
)

type Peer struct {
	Conn    *grpc.ClientConn
	Enabled bool
}

type Gossip struct {
	id                  uint64
	mtx                 sync.RWMutex
	healthcheckerCtx    context.Context
	healthcheckerCancel context.CancelFunc
	healthcheckerDone   chan struct{}
	rpcDialer           rpc.Dialer
	peers               map[uint64]*Peer
	mlist               *memberlist.Memberlist
	logger              *zap.Logger
	meta                []byte
}

func (m *Gossip) Members() []*api.Member {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	out := make([]*api.Member, len(m.peers))
	idx := 0
	for id, peer := range m.peers {
		out[idx] = &api.Member{ID: id, Address: peer.Conn.Target(), IsAlive: peer.Enabled}
		idx++
	}
	return out
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
	self.mtx.Lock()
	defer self.mtx.Unlock()
	self.logger.Debug("stopping membership healthchecker")
	self.healthcheckerCancel()
	self.logger.Debug("closing peers RPC connections")
	for id, conn := range self.peers {
		conn.Conn.Close()
		delete(self.peers, id)
	}
	self.logger.Info("stopped peers RPC connexions")
	<-self.healthcheckerDone
	self.logger.Info("stopped membership healthchecker")
	self.logger.Debug("leaving mesh")
	err := self.mlist.Leave(1 * time.Second)
	if err != nil {
		self.logger.Error("failed to leave mesh", zap.Error(err))
		return err
	}
	self.logger.Info("left mesh")
	return self.mlist.Shutdown()
}

func (self *Gossip) MemberCount() int {
	if self.mlist == nil {
		return 1
	}
	return self.mlist.NumMembers()
}

func New(id uint64, port int, advertiseAddr string, advertisePort, rpcPort int, dialer rpc.Dialer, logger *zap.Logger) *Gossip {
	idBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(idBuf, id)
	idstr := string(idBuf)
	self := &Gossip{
		id:                id,
		peers:             make(map[uint64]*Peer),
		meta:              EncodeMD(id, fmt.Sprintf("%s:%d", advertiseAddr, rpcPort)),
		rpcDialer:         dialer,
		logger:            logger,
		healthcheckerDone: make(chan struct{}),
	}

	self.healthcheckerCtx, self.healthcheckerCancel = context.WithCancel(context.Background())
	go func() {
		defer close(self.healthcheckerDone)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-self.healthcheckerCtx.Done():
				return
			case <-ticker.C:
				self.runHealthchecks(self.healthcheckerCtx)
			}
		}
	}()
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
