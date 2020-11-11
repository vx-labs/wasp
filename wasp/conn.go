package wasp

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vx-labs/mqtt-protocol/decoder"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/auth"
	"github.com/vx-labs/wasp/wasp/epoll"
	"github.com/vx-labs/wasp/wasp/sessions"
	"github.com/vx-labs/wasp/wasp/stats"
	"github.com/vx-labs/wasp/wasp/transport"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type manager struct {
	authHandler     AuthenticationHandler
	fsm             FSM
	state           ReadState
	writer          Writer
	setupJobs       chan chan transport.Metadata
	connectionsJobs chan chan *epoll.ClientConn
	epoll           *epoll.Epoll
	publishHandler  PublishHandler
	wg              *sync.WaitGroup
}

type setupWorker struct {
	decoder     *decoder.Sync
	encoder     *encoder.Encoder
	authHandler AuthenticationHandler
	fsm         FSM
	state       ReadState
	writer      Writer
	epoll       *epoll.Epoll
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

func NewConnectionManager(authHandler AuthenticationHandler, fsm FSM, state ReadState, writer Writer, publishHandler PublishHandler) *manager {

	epoller, err := epoll.New(connWorkers)
	if err != nil {
		panic(err)
	}

	s := &manager{
		authHandler:     authHandler,
		fsm:             fsm,
		state:           state,
		setupJobs:       make(chan chan transport.Metadata, setuppers),
		connectionsJobs: make(chan chan *epoll.ClientConn, connWorkers),
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
			s.epoll.Expire(t)
		}
	}
}

func (s *manager) runDispatcher(ctx context.Context) {
	defer s.wg.Done()
	connections := make([]*epoll.ClientConn, connWorkers)
	for {
		n, err := s.epoll.Wait(connections)
		if err != nil && err != unix.EINTR {
			return
		}
		for _, c := range connections[:n] {
			if c != nil {
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
}
func (s *manager) runSetupper(ctx context.Context) {
	worker := &setupWorker{
		decoder:     decoder.New(),
		encoder:     encoder.New(),
		authHandler: s.authHandler,
		fsm:         s.fsm,
		state:       s.state,
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
	for _, session := range s.state.ListSessions() {
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
		ch := make(chan *epoll.ClientConn)
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
				err := worker.processConn(ctx, job)
				if err != nil {
					job.Conn.Close()
				}
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
	session, err := sessions.NewSession(ctx, id, mountPoint, m.Name, c, connectPkt)
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
	if metadata := s.state.GetSessionMetadatasByClientID(session.ClientID); metadata != nil {
		err := s.fsm.DeleteSessionMetadata(ctx, metadata.SessionID, metadata.MountPoint)
		if err != nil {
			return err
		}
		L(ctx).Debug("deleted old session metadata")
	}
	err = s.fsm.CreateSessionMetadata(ctx, session.ID, session.ClientID, session.LWT(), session.MountPoint)
	if err != nil {
		L(ctx).Error("failed to create session metadata", zap.Error(err))
		return err
	}
	L(ctx).Debug("session metadata created")
	s.state.SaveSession(session.ID, session)
	s.writer.Register(session.ID, c)
	err = s.epoll.Add(&epoll.ClientConn{ID: id, FD: m.FD, Conn: c, Deadline: session.NextDeadline(time.Now())})
	if err != nil {
		L(ctx).Error("failed to register epoll session", zap.Error(err))
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

func (s *connectionWorker) processConn(ctx context.Context, c *epoll.ClientConn) error {
	session := s.manager.state.GetSession(c.ID)
	if session == nil {
		c.Conn.Close()
		return nil
	}
	err := s.processSession(ctx, session, c)
	if err != nil {
		if err == ErrSessionDisconnected {
			session.Disconnected = true
		}
		s.manager.shutdownSession(ctx, session)
		s.manager.epoll.Remove(c.FD)
		return err
	}
	return err
}
func (s *manager) shutdownSession(ctx context.Context, session *sessions.Session) {
	s.writer.Unregister(session.ID)
	s.state.CloseSession(session.ID)
	topics := session.GetTopics()
	for idx := range topics {
		s.fsm.Unsubscribe(ctx, session.ID, topics[idx])
	}
	metadata := s.state.GetSessionMetadatasByClientID(session.ClientID)
	s.fsm.DeleteSessionMetadata(ctx, session.ID, session.MountPoint)
	if metadata == nil || metadata.SessionID != session.ID || session.Disconnected {
		// Session has reconnected on another peer.
		return
	}
	if !session.Disconnected {
		L(ctx).Debug("session lost")
		if session.LWT() != nil {
			err := s.publishHandler(ctx, session.ID, session.LWT())
			if err != nil {
				L(ctx).Warn("failed to publish session LWT", zap.Error(err))
			}
		}
	}
}

type timeoutError interface {
	Timeout() bool
}

func (s *connectionWorker) processSession(ctx context.Context, session *sessions.Session, c *epoll.ClientConn) error {
	started := time.Now()
	pkt, err := s.decoder.Decode(c.Conn)
	if err != nil {
		return err
	}
	defer stats.SessionPacketHandling.With(prometheus.Labels{
		"packet_type": packet.TypeString(pkt),
	}).Observe(stats.MilisecondsElapsed(started))

	stats.IngressBytes.With(map[string]string{
		"protocol": session.Transport(),
	}).Add(float64(pkt.Length()))

	s.manager.epoll.SetDeadline(c.FD, session.NextDeadline(time.Now()))
	s.manager.epoll.Rearm(c.FD)
	err = processPacket(ctx, s.manager.fsm, s.manager.state, s.manager.publishHandler, s.manager.writer, session, s.encoder, c.Conn, pkt)
	if err != nil {
		return err
	}
	return nil
}
