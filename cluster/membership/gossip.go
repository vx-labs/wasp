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
	api "github.com/vx-labs/wasp/cluster/clusterpb"
	"google.golang.org/grpc"

	"go.uber.org/zap"
)

type Peer struct {
	Conn       *grpc.ClientConn
	LastUpdate time.Time
	Enabled    bool
}

type Gossip struct {
	id                  uint64
	mtx                 sync.RWMutex
	healthcheckerCtx    context.Context
	healthcheckerCancel context.CancelFunc
	healthcheckerDone   chan struct{}
	rpcDialer           func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	peers               map[uint64]*Peer
	mlist               *memberlist.Memberlist
	logger              *zap.Logger
	meta                []byte
}

func (m *Gossip) Members() []*api.Member {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	out := make([]*api.Member, len(m.peers)+1)
	idx := 0
	for id, peer := range m.peers {
		out[idx] = &api.Member{ID: id, Address: peer.Conn.Target(), IsAlive: peer.Enabled}
		idx++
	}
	selfMD, err := DecodeMD(m.meta)
	if err != nil {
		out[idx] = &api.Member{ID: m.id, Address: m.mlist.LocalNode().FullAddress().Addr, IsAlive: true}
	} else {
		out[idx] = &api.Member{ID: m.id, Address: selfMD.RPCAddress, IsAlive: true}
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
	self.logger.Debug("stopped peers RPC connexions")
	<-self.healthcheckerDone
	self.logger.Debug("stopped membership healthchecker")
	self.logger.Debug("leaving mesh")
	err := self.mlist.Leave(1 * time.Second)
	if err != nil {
		self.logger.Error("failed to leave mesh", zap.Error(err))
		return err
	}
	self.logger.Debug("left mesh")
	return self.mlist.Shutdown()
}

func (self *Gossip) MemberCount() int {
	if self.mlist == nil {
		return 1
	}
	return self.mlist.NumMembers()
}

func New(id uint64, clusterName string, port int, advertiseAddr string, advertisePort, rpcPort int, dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error), logger *zap.Logger) *Gossip {
	idBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(idBuf, id)
	idstr := string(idBuf)
	self := &Gossip{
		id:                id,
		peers:             make(map[uint64]*Peer),
		meta:              EncodeMD(id, clusterName, fmt.Sprintf("%s:%d", advertiseAddr, rpcPort)),
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
