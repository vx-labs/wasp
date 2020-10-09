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

type pool struct {
	id                  uint64
	mtx                 sync.RWMutex
	healthcheckerCtx    context.Context
	healthcheckerCancel context.CancelFunc
	healthcheckerDone   chan struct{}
	rpcDialer           func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	peers               map[uint64]*member
	mlist               *memberlist.Memberlist
	logger              *zap.Logger
	recorder            Recorder
	meta                []byte
}

func (p *pool) DoesMemberExists(id uint64) bool {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	_, ok := p.peers[id]
	return ok
}

func (p *pool) Members() []*api.Member {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	out := make([]*api.Member, len(p.peers)+1)
	idx := 0
	for id, peer := range p.peers {
		out[idx] = &api.Member{ID: id, Address: peer.Conn.Target(), IsAlive: peer.Enabled}
		idx++
	}
	pMD, err := DecodeMD(p.meta)
	if err != nil {
		out[idx] = &api.Member{ID: p.id, Address: p.mlist.LocalNode().FullAddress().Addr, IsAlive: true}
	} else {
		out[idx] = &api.Member{ID: p.id, Address: pMD.RPCAddress, IsAlive: true}
	}
	return out
}

func (p *pool) UseRecorder(recorder Recorder) {
	p.recorder = recorder
}
func (p *pool) NodeMeta(limit int) []byte {
	return p.meta
}

func (p *pool) Join(hosts []string) error {
	if len(hosts) == 0 {
		return nil
	}
	_, err := p.mlist.Join(hosts)
	return err
}
func (p *pool) UpdateMetadata(meta []byte) {
	p.meta = meta
	p.mlist.UpdateNode(5 * time.Second)
}
func (p *pool) Shutdown() error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.logger.Debug("stopping membership healthchecker")
	p.healthcheckerCancel()
	p.logger.Debug("closing peers RPC connections")
	for id, conn := range p.peers {
		conn.Conn.Close()
		delete(p.peers, id)
	}
	p.logger.Debug("stopped peers RPC connexions")
	<-p.healthcheckerDone
	p.logger.Debug("stopped membership healthchecker")
	p.logger.Debug("leaving mesh")
	err := p.mlist.Leave(1 * time.Second)
	if err != nil {
		p.logger.Error("failed to leave mesh", zap.Error(err))
		return err
	}
	p.logger.Debug("left mesh")
	return p.mlist.Shutdown()
}

func (p *pool) MemberCount() int {
	if p.mlist == nil {
		return 1
	}
	return p.mlist.NumMembers()
}

// New creates a new membership pool.
func New(id uint64, clusterName string, port int, advertiseAddr string, advertisePort, rpcPort int, dialer func(address string, opts ...grpc.DialOption) (*grpc.ClientConn, error), logger *zap.Logger) Pool {
	idBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(idBuf, id)
	idstr := string(idBuf)
	p := &pool{
		id:                id,
		peers:             make(map[uint64]*member),
		meta:              EncodeMD(id, clusterName, fmt.Sprintf("%s:%d", advertiseAddr, rpcPort)),
		rpcDialer:         dialer,
		logger:            logger,
		healthcheckerDone: make(chan struct{}),
	}

	p.healthcheckerCtx, p.healthcheckerCancel = context.WithCancel(context.Background())
	go func() {
		defer close(p.healthcheckerDone)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-p.healthcheckerCtx.Done():
				return
			case <-ticker.C:
				p.runHealthchecks(p.healthcheckerCtx)
			}
		}
	}()
	config := memberlist.DefaultLANConfig()
	config.AdvertiseAddr = advertiseAddr
	config.AdvertisePort = advertisePort
	config.BindPort = port
	config.BindAddr = "0.0.0.0"
	config.Name = idstr
	config.Delegate = p
	config.Events = p
	if os.Getenv("ENABLE_MEMBERLIST_LOG") != "true" {
		config.LogOutput = ioutil.Discard
	}
	list, err := memberlist.Create(config)
	if err != nil {
		panic(err)
	}
	p.mlist = list
	return p
}
