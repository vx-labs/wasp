package wasp

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/vx-labs/mqtt-protocol/decoder"
	"github.com/vx-labs/mqtt-protocol/encoder"
	"github.com/vx-labs/mqtt-protocol/packet"
	"github.com/vx-labs/wasp/wasp/auth"
	"github.com/vx-labs/wasp/wasp/epoll"
	"github.com/vx-labs/wasp/wasp/sessions"
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
	connectionsJobs chan chan *epoll.Session
	epoll           *epoll.Epoll
	publishHandler  PublishHandler
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
	decoder        *decoder.Sync
	encoder        *encoder.Encoder
	fsm            FSM
	state          ReadState
	writer         Writer
	epoll          *epoll.Epoll
	publishHandler PublishHandler
}

const (
	connectTimeout time.Duration = 3 * time.Second
)

func NewConnectionManager(ctx context.Context, authHandler AuthenticationHandler, fsm FSM, state ReadState, writer Writer, publishHandler PublishHandler) *manager {
	setuppers := 20
	connWorkers := 50

	epoller, err := epoll.New()
	if err != nil {
		panic(err)
	}

	s := &manager{
		authHandler:     authHandler,
		fsm:             fsm,
		state:           state,
		setupJobs:       make(chan chan transport.Metadata, setuppers),
		connectionsJobs: make(chan chan *epoll.Session, connWorkers),
		writer:          writer,
		publishHandler:  publishHandler,
		epoll:           epoller,
	}

	for i := 0; i < setuppers; i++ {
		s.runSetupper(ctx)
	}
	for i := 0; i < connWorkers; i++ {
		s.runConnWorker(ctx)
	}
	go s.runDispatcher(ctx)
	return s
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
func (s *manager) runDispatcher(ctx context.Context) {
	for {
		connections, err := s.epoll.Wait()
		if err != nil && err != unix.EINTR {
			log.Printf("Failed to epoll wait %v", err)
			continue
		}
		for _, c := range connections {
			if c != nil {
				s.epoll.Remove(c.FD)
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

func (s *manager) runConnWorker(ctx context.Context) {
	worker := &connectionWorker{
		decoder:        decoder.New(),
		encoder:        encoder.New(),
		fsm:            s.fsm,
		state:          s.state,
		writer:         s.writer,
		epoll:          s.epoll,
		publishHandler: s.publishHandler,
	}
	go func() {
		ch := make(chan *epoll.Session)
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
				} else {
					s.epoll.Add(job.ID, job.FD, job.Conn)
				}
			}
		}
	}()
}

func (s *setupWorker) setup(ctx context.Context, m transport.Metadata) error {
	c := m.Channel
	c.SetDeadline(
		time.Now().Add(connectTimeout),
	)
	firstPkt, err := s.decoder.Decode(c)
	if err != nil || firstPkt == nil {
		return err
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
	session, err := sessions.NewSession(ctx, id, mountPoint, c, connectPkt)
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
		return err
	}
	L(ctx).Debug("session metadata created")
	s.state.SaveSession(session.ID, session)
	c.SetDeadline(
		session.NextDeadline(time.Now()),
	)
	s.writer.Register(session.ID, c)
	s.epoll.Add(id, m.FD, c)
	return s.encoder.ConnAck(c, &packet.ConnAck{
		Header:     connectPkt.Header,
		ReturnCode: packet.CONNACK_CONNECTION_ACCEPTED,
	})
}

func (s *connectionWorker) processConn(ctx context.Context, c *epoll.Session) error {
	session := s.state.GetSession(c.ID)
	if session == nil {
		c.Conn.Close()
		return nil
	}
	err := s.processSession(ctx, session, c)
	if err != nil {
		if err == ErrSessionDisconnected {
			session.Disconnected = true
		}
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
			return errors.New("session reconnected on anoher host")
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
	} else {
		c.Conn.SetDeadline(session.NextDeadline(time.Now()))
	}
	return err
}
func (s *connectionWorker) processSession(ctx context.Context, session *sessions.Session, c *epoll.Session) error {
	pkt, err := s.decoder.Decode(c.Conn)
	if err != nil {
		return err
	}
	return processPacket(ctx, s.fsm, s.state, s.publishHandler, s.writer, session, s.encoder, c.Conn, pkt)
}
