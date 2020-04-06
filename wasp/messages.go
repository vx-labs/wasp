package wasp

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"path"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/pb"
	"github.com/gogo/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
	"go.uber.org/zap"
)

var (
	localMessagesBucketName  []byte = []byte("local")
	remoteMessagesBucketName []byte = []byte("remote")
	encoding                        = binary.BigEndian
)

type MessageLog interface {
	io.Closer
	Append(b [][]byte) (err error)
	Consume(context.Context, uint64, func(*packet.Publish) error) error
	GC()
}

type messageLog struct {
	db       *badger.DB
	firstSeq uint64
}

type compatLogger struct {
	l *zap.Logger
}

func (c *compatLogger) Debugf(string, ...interface{})   {}
func (c *compatLogger) Infof(string, ...interface{})    {}
func (c *compatLogger) Warningf(string, ...interface{}) {}
func (c *compatLogger) Errorf(string, ...interface{})   {}

func NewMessageLog(ctx context.Context, datadir string) (MessageLog, error) {
	opts := badger.DefaultOptions(path.Join(datadir, "badger"))
	opts.Logger = &compatLogger{l: L(ctx)}
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	seq, err := db.GetSequence(localMessagesBucketName, 1)
	if err != nil {
		return nil, err
	}
	offset, err := seq.Next()
	if err != nil {
		return nil, err
	}
	return &messageLog{
		db:       db,
		firstSeq: offset,
	}, seq.Release()
}

func (s *messageLog) GC() {
again:
	err := s.db.RunValueLogGC(0.7)
	if err == nil {
		goto again
	}
}
func (s *messageLog) Close() error {
	return s.db.Close()
}
func (s *messageLog) Append(b [][]byte) error {
	tx := s.db.NewTransaction(true)
	defer tx.Discard()

	seq, err := s.db.GetSequence(localMessagesBucketName, uint64(len(b)))
	if err != nil {
		return err
	}
	defer seq.Release()

	for idx := range b {
		offset, err := seq.Next()
		if err != nil {
			return err
		}
		entry := badger.NewEntry(int64ToBytes(offset), b[idx]).WithTTL(2 * time.Hour)
		err = tx.SetEntry(entry)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func bytesToInt64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

func int64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
}

func (s *messageLog) GetRecords(ctx context.Context, fromOffset uint64, f func(*packet.Publish) error) (uint64, error) {
	stream := s.db.NewStream()
	stream.Prefix = []byte{0}
	stream.ChooseKey = func(item *badger.Item) bool {
		return fromOffset < bytesToInt64(item.Key())
	}
	var lastRecord uint64 = fromOffset
	stream.Send = func(list *pb.KVList) error {
		for _, kv := range list.Kv {
			record := &packet.Publish{}
			err := proto.Unmarshal(kv.Value, record)
			if err != nil {
				return err
			}

			err = f(record)
			if err != nil {
				return err
			}
			lastRecord = bytesToInt64(kv.Key)
		}
		return nil
	}
	err := stream.Orchestrate(ctx)
	return lastRecord, err
}

func (s *messageLog) Consume(ctx context.Context, fromOffset uint64, f func(*packet.Publish) error) error {
	notifications := make(chan struct{}, 1)

	notifications <- struct{}{}
	defer close(notifications)

	go func() {
		var lastSeen uint64 = fromOffset
		var err error
		for range notifications {
			lastSeen, err = s.GetRecords(ctx, lastSeen, f)
			if err != nil {
				log.Print(err) // TODO: use logger
			}
		}
	}()

	return s.db.Subscribe(ctx, func(list *pb.KVList) error {
		select {
		case notifications <- struct{}{}:
		default:
		}
		return nil
	}, nil)
}
