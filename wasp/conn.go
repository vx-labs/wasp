package wasp

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vx-labs/mqtt-protocol/decoder"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/ack"
	"github.com/vx-labs/wasp/wasp/auth"
	"github.com/vx-labs/wasp/wasp/distributed"
	"github.com/vx-labs/wasp/wasp/epoll"
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/transport"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type manager struct {
	authHandler     AuthenticationHandler
	state           distributed.State
	local           ReadState
	writer          Writer
	setupJobs       chan chan transport.Metadata
	connectionsJobs chan chan epoll.Event
	epoll           epoll.Instance
	publishHandler  PublishHandler
	inflights       ack.Queue
	wg              *sync.WaitGroup
}

type setupWorker struct {
	decoder     *decoder.Sync
	encoder     *encoder.Encoder
	authHandler AuthenticationHandler
	state       distributed.State
	local       ReadState
	writer      Writer
	epoll       epoll.Instance
}

type connectionWorker struct {
	decoder *decoder.Sync
	encoder *encoder.Encoder
	manager *manager
}

const (
	connectTimeout time.Duration = 3 * time.Second
)

var (
	setuppers   int = 20
	connWorkers int = 50
)

func NewConnectionManager(authHandler AuthenticationHandler, local ReadState, state distributed.State, writer Writer, publishHandler PublishHandler, ackQueue ack.Queue) *manager {

	epoller, err := epoll.NewInstance(connWorkers)
	if err != nil {
		panic(err)
	}

	s := &manager{
		authHandler:     authHandler,
		inflights:       ackQueue,
		state:           state,
		local:           local,
		setupJobs:       make(chan chan transport.Metadata, setuppers),
		connectionsJobs: make(chan chan epoll.Event, connWorkers),
		writer:          writer,
		publishHandler:  publishHandler,
		epoll:           epoller,
		wg:              &sync.WaitGroup{},
	}
	return s
}
func (s *manager) Run(ctx context.Context) {
	for i := 0; i < setuppers; i++ {
		s.runSetupper(ctx)
		s.wg.Add(1)
	}
	for i := 0; i < connWorkers; i++ {
		s.runConnWorker(ctx)
		s.wg.Add(1)
	}
	go s.runTimeouter(ctx)
	go s.runDispatcher(ctx)
	s.wg.Add(2)
	<-ctx.Done()
	s.epoll.Shutdown()
	s.wg.Wait()
}

func (s *manager) Setup(ctx context.Context, c transport.Metadata) {
	select {
	case <-ctx.Done():
		return
	case runner := <-s.setupJobs:
		select {
		case <-ctx.Done():
			return
		case runner <- c:
			return
		}
	}
}
func (s *manager) runTimeouter(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			for _, conn := range s.epoll.Expire(t) {
				session := s.local.GetSession(conn.ID)
				if session != nil {
					s.shutdownSession(ctx, session)
				}
			}
		}
	}
}

func (s *manager) runDispatcher(ctx context.Context) {
	defer s.wg.Done()
	connections := make([]epoll.Event, connWorkers)
	for {
		n, err := s.epoll.Wait(connections)
		if err != nil && err != unix.EINTR {
			return
		}
		for _, c := range connections[:n] {
			select {
			case <-ctx.Done():
				return
			case runner := <-s.connectionsJobs:
				select {
				case <-ctx.Done():
					return
				case runner <- c:
				}
			}
		}
	}
}
func (s *manager) runSetupper(ctx context.Context) {
	worker := &setupWorker{
		decoder:     decoder.New(),
		encoder:     encoder.New(),
		authHandler: s.authHandler,
		state:       s.state,
		local:       s.local,
		writer:      s.writer,
		epoll:       s.epoll,
	}
	go func() {
		defer s.wg.Done()
		ch := make(chan transport.Metadata)
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case s.setupJobs <- ch:
			}
			select {
			case <-ctx.Done():
				return
			case job := <-ch:
				err := worker.setup(ctx, job)
				if err != nil {
					job.Channel.Close()
				}
			}
		}
	}()
}

func (s *manager) DisconnectClients(ctx context.Context) {
	for _, session := range s.local.ListSessions() {
		s.shutdownSession(ctx, session)
	}
}
func (s *manager) runConnWorker(ctx context.Context) {
	worker := &connectionWorker{
		decoder: decoder.New(),
		encoder: encoder.New(),
		manager: s,
	}
	go func() {
		defer s.wg.Done()
		ch := make(chan epoll.Event)
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case s.connectionsJobs <- ch:
			}
			select {
			case <-ctx.Done():
				return
			case job := <-ch:
				worker.processConn(ctx, job)
			}
		}
	}()
}

func (s *setupWorker) setup(ctx context.Context, m transport.Metadata) error {
	c := m.Channel
	c.SetReadDeadline(
		time.Now().Add(connectTimeout),
	)
	firstPkt, err := s.decoder.Decode(c)
	if err != nil {
		return err
	}
	if firstPkt == nil {
		return ErrConnectNotDone
	}
	connectPkt, ok := firstPkt.(*packet.Connect)
	if !ok {
		return ErrConnectNotDone
	}

	principal, err := s.authHandler.Authenticate(ctx, auth.ApplicationContext{
		ClientID: connectPkt.ClientId,
		Username: connectPkt.Username,
		Password: connectPkt.Password,
	}, auth.TransportContext{
		Encrypted:       m.Encrypted,
		RemoteAddress:   m.RemoteAddress,
		X509Certificate: nil,
	})
	id := principal.ID
	mountPoint := principal.MountPoint
	if err != nil {
		L(ctx).Info("authentication failed", zap.Error(err))
		return s.encoder.ConnAck(c, &packet.ConnAck{
			Header:     connectPkt.Header,
			ReturnCode: packet.CONNACK_REFUSED_BAD_USERNAME_OR_PASSWORD,
		})
	}
	session, err := sessions.NewSession(id, mountPoint, m.Name, connectPkt)
	if err != nil {
		return err
	}

	ctx = AddFields(ctx,
		zap.String("session_id", id),
		zap.String("session_mountpoint", mountPoint),
		zap.Time("connected_at", time.Now()),
		zap.String("session_username", string(connectPkt.Username)),
	)
	L(ctx).Debug("session connected")
	if metadata, err := s.state.SessionMetadatas().ByClientID(session.ClientID()); err == nil {
		err := s.state.SessionMetadatas().Delete(metadata.SessionID)
		if err != nil {
			return err
		}
		L(ctx).Debug("deleted old session metadata")
	}
	err = s.state.SessionMetadatas().Create(session.ID(), session.ClientID(), time.Now().Unix(), session.LWT(), session.MountPoint())
	if err != nil {
		L(ctx).Error("failed to create session metadata", zap.Error(err))
		return err
	}
	L(ctx).Debug("session metadata created")
	s.local.SaveSession(session.ID(), session)
	s.writer.Register(session.ID(), c)
	err = s.epoll.Add(epoll.ClientConn{ID: id, FD: m.FD, Conn: c, Deadline: session.NextDeadline(time.Now())})
	if err != nil {
		L(ctx).Error("failed to register epoll session", zap.Error(err), zap.Int("conn_fd", m.FD))
		return err
	}
	c.SetReadDeadline(
		time.Time{},
	)
	return s.encoder.ConnAck(c, &packet.ConnAck{
		Header:     connectPkt.Header,
		ReturnCode: packet.CONNACK_CONNECTION_ACCEPTED,
	})
}

func (s *connectionWorker) processConn(ctx context.Context, ev epoll.Event) {
	session := s.manager.local.GetSession(ev.ID)
	if session == nil {
		s.manager.epoll.Remove(ev.FD)
		return
	}
	if ev.Event&unix.EPOLLRDHUP == unix.EPOLLRDHUP || !s.processSession(ctx, session, ev.Conn) {
		s.manager.shutdownSession(ctx, session)
		err := s.manager.epoll.Remove(ev.FD)
		if err != nil {
			L(ctx).Warn("failed to remove session from epoll tracking", zap.Error(err))
		}
	} else {
		s.manager.epoll.SetDeadline(ev.FD, session.NextDeadline(time.Now()))
		s.manager.epoll.Rearm(ev.FD)
	}
}
func (s *manager) shutdownSession(ctx context.Context, session *sessions.Session) {
	s.writer.Unregister(session.ID())
	s.local.CloseSession(session.ID())
	topics := session.GetTopics()
	for idx := range topics {
		s.state.Subscriptions().Delete(session.ID(), topics[idx])
	}
	metadata, err := s.state.SessionMetadatas().ByClientID(session.ClientID())
	if err == nil {
		if metadata.SessionID != session.ID() || session.Disconnected {
			// Session has reconnected on another peer.
			return
		}
		s.state.SessionMetadatas().Delete(session.ID())
	}
	if !session.Disconnected {
		L(ctx).Debug("session lost")
		if lwt := session.LWT(); lwt != nil {
			err := s.publishHandler(ctx, session.ID(), lwt)
			if err != nil {
				L(ctx).Warn("failed to publish session LWT", zap.Error(err))
			}
		}
	}
}

type timeoutError interface {
	Timeout() bool
}

func (s *connectionWorker) processSession(ctx context.Context, session *sessions.Session, c io.ReadWriter) bool {
	started := time.Now()
	pkt, err := s.decoder.Decode(c)
	if err != nil {
		return false
	}
	defer stats.SessionPacketHandling.With(prometheus.Labels{
		"packet_type": packet.TypeString(pkt),
	}).Observe(stats.MilisecondsElapsed(started))

	stats.IngressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(pkt.Length()))

	err = processPacket(ctx, s.manager.local, s.manager.state, s.manager.publishHandler, s.manager.writer, s.manager.inflights, session, s.encoder, c, pkt)
	if err != nil {
		if err == ErrSessionDisconnected {
			session.Disconnected = true
		}
		return false
	}
	return true
}
