package wasp

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vx-labs/mqtt-protocol/decoder"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/v4/wasp/ack"
	"github.com/vx-labs/wasp/v4/wasp/auth"
	"github.com/vx-labs/wasp/v4/wasp/distributed"
	"github.com/vx-labs/wasp/v4/wasp/sessions"
	"github.com/vx-labs/wasp/v4/wasp/stats"
	"github.com/vx-labs/wasp/v4/wasp/transport"

	"go.uber.org/zap"
)

type Manager interface {
	Run(ctx context.Context)
	Setup(ctx context.Context, c transport.Metadata)
	DisconnectClients(ctx context.Context)
}

type manager struct {
	authHandler     AuthenticationHandler
	state           distributed.State
	local           LocalState
	writer          Writer
	setupJobs       chan chan transport.Metadata
	inflights       ack.Queue
	wg              *sync.WaitGroup
	packetProcessor PacketProcessor
}

type setupWorker struct {
	decoder     *decoder.Sync
	manager     *manager
	encoder     *encoder.Encoder
	authHandler AuthenticationHandler
	state       distributed.State
	local       LocalState
	writer      Writer
}

type connectionWorker struct {
	decoder *decoder.Sync
	manager *manager
}

const (
	connectTimeout time.Duration = 3 * time.Second
)

var (
	setuppers int = 20
)

func NewConnectionManager(authHandler AuthenticationHandler, local LocalState, state distributed.State, writer Writer, packetProcesor PacketProcessor, ackQueue ack.Queue) Manager {

	s := &manager{
		authHandler:     authHandler,
		inflights:       ackQueue,
		state:           state,
		local:           local,
		setupJobs:       make(chan chan transport.Metadata, setuppers),
		writer:          writer,
		packetProcessor: packetProcesor,
		wg:              &sync.WaitGroup{},
	}
	return s
}
func (s *manager) Run(ctx context.Context) {
	for i := 0; i < setuppers; i++ {
		s.runSetupper(ctx)
		s.wg.Add(1)
	}
	<-ctx.Done()
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

func (s *manager) runSetupper(ctx context.Context) {
	worker := &setupWorker{
		manager:     s,
		decoder:     decoder.New(),
		encoder:     encoder.New(),
		authHandler: s.authHandler,
		state:       s.state,
		local:       s.local,
		writer:      s.writer,
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
	transportCtx := auth.TransportContext{
		Encrypted:            m.Encrypted,
		RemoteAddress:        m.RemoteAddress,
		X509CertificateChain: nil,
	}
	if m.EncryptionState != nil {
		for _, cert := range m.EncryptionState.PeerCertificates {
			transportCtx.X509CertificateChain = append(transportCtx.X509CertificateChain, cert.Raw)
		}
	}
	principal, err := s.authHandler.Authenticate(ctx, auth.ApplicationContext{
		ClientID: connectPkt.ClientId,
		Username: connectPkt.Username,
		Password: connectPkt.Password,
	}, transportCtx)
	id := principal.ID
	mountPoint := principal.MountPoint
	if err != nil {
		L(ctx).Info("authentication failed", zap.Error(err))
		return s.encoder.ConnAck(c, &packet.ConnAck{
			Header:     connectPkt.Header,
			ReturnCode: packet.CONNACK_REFUSED_BAD_USERNAME_OR_PASSWORD,
		})
	}
	session, err := sessions.NewSession(id, mountPoint, m.Name, m.Channel, connectPkt)
	if err != nil {
		return err
	}

	ctx = AddFields(ctx,
		zap.String("session_id", id),
		zap.String("client_id", session.ClientID()),
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
	s.local.Create(session.ID(), session)
	worker := &connectionWorker{
		decoder: decoder.New(),
		manager: s.manager,
	}
	go worker.serve(ctx, session)
	return s.encoder.ConnAck(c, &packet.ConnAck{
		Header:     connectPkt.Header,
		ReturnCode: packet.CONNACK_CONNECTION_ACCEPTED,
	})
}

func (s *connectionWorker) serve(ctx context.Context, session *sessions.Session) {
	sessionCtx, cancel := context.WithCancel(ctx)
	for s.processSession(sessionCtx, session) {
		session.ExtendDeadline()
	}
	cancel()
	s.manager.shutdownSession(ctx, session)
}

func (s *manager) shutdownSession(ctx context.Context, session *sessions.Session) {
	s.local.Delete(session.ID())
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
			err := s.packetProcessor.Process(ctx, session, nil, lwt)
			if err != nil {
				L(ctx).Warn("failed to publish session LWT", zap.Error(err))
			}
		}
	}
}

type timeoutError interface {
	Timeout() bool
}

func (s *connectionWorker) processSession(ctx context.Context, session *sessions.Session) bool {
	c := session.ReadWriter()
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

	err = s.manager.packetProcessor.Process(ctx, session, c, pkt)
	if err != nil {
		if err == ErrSessionDisconnected {
			session.Disconnected = true
		}
		return false
	}
	return true
}
