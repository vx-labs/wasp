package wasp

import (
	"context"
	"encoding/binary"
	"io"
	"path"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/pb"
	"github.com/golang/protobuf/proto"
	"github.com/vx-labs/mqtt-protocol/packet"
	"go.uber.org/zap"
)

var (
	messageBucketName []byte = []byte("messages")
	encoding                 = binary.BigEndian
)

type MessageLog interface {
	io.Closer
	Append(b [][]byte) (err error)
	Consume(context.Context, func(*packet.Publish)) error
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
	seq, err := db.GetSequence(messageBucketName, 1)
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
	seq, err := s.db.GetSequence(messageBucketName, 100)
	if err != nil {
		return err
	}
	defer seq.Release()

	defer tx.Discard()
	for idx := range b {
		offset, err := seq.Next()
		if err != nil {
			return err
		}
		err = tx.Set(int64ToBytes(uint64(offset)), b[idx])
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

func (s *messageLog) Consume(ctx context.Context, f func(*packet.Publish)) error {
	return s.db.Subscribe(ctx, func(list *pb.KVList) {
		for _, kv := range list.Kv {
			value := &packet.Publish{}
			err := proto.Unmarshal(kv.GetValue(), value)
			if err != nil {
				continue
			}
			f(value)
		}
	}, []byte{0})
}
