package messages

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/tysontate/gommap"
	"github.com/vx-labs/commitlog"
	"github.com/vx-labs/commitlog/stream"
	"github.com/vx-labs/mqtt-protocol/packet"
)

var (
	// Encoding is the default binary encoding
	Encoding = binary.BigEndian
	// ErrKeyNotFound is an error indicating a given key does not exist
	ErrKeyNotFound = errors.New("not found")
	// ErrBucketNotFound is an error indicating a given key does not exist
	ErrBucketNotFound = errors.New("bucket not found")
	// ErrIndexOutdated is an error indicating that the supplied index is outdated
	ErrIndexOutdated        = errors.New("index outdated")
	maxMessageCount  uint64 = 5000
)

func mustEncode(p *packet.Publish) []byte {
	buf, err := proto.Marshal(p)
	if err != nil {
		panic(err)
	}
	return buf
}
func mustDecode(b []byte) *packet.Publish {
	p := &packet.Publish{}
	err := proto.Unmarshal(b, p)
	if err != nil {
		panic(err)
	}
	return p
}

type Log interface {
	io.Closer
	Append(payload *packet.Publish) error
	Consume(ctx context.Context, consumerName string, f func(*packet.Publish) error) error
}

type store struct {
	datadir string
	log     commitlog.CommitLog
}

func New(datadir string) (Log, error) {
	log, err := commitlog.Open(path.Join(datadir, "log"), 250)
	if err != nil {
		return nil, err
	}
	return &store{log: log, datadir: datadir}, nil
}

func (s *store) Close() error { return s.log.Close() }

func (s *store) Append(publish *packet.Publish) error {
	_, err := s.log.WriteEntry(uint64(time.Now().UnixNano()), mustEncode(publish))
	return err
}

func (s *store) Consume(ctx context.Context, consumerName string, f func(*packet.Publish) error) error {

	statePath := path.Join(s.datadir, fmt.Sprintf("%s.state", consumerName))
	var fd *os.File

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		fd, err = os.OpenFile(statePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0650)
		if err != nil {
			return err
		}
		err = fd.Truncate(8)
		if err != nil {
			fd.Close()
			os.Remove(statePath)
			return err
		}
	} else {
		fd, err = os.OpenFile(statePath, os.O_RDWR, 0650)
		if err != nil {
			return err
		}
	}
	stateOffset, err := gommap.Map(fd.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED)
	if err != nil {
		return err
	}
	defer func() {
		stateOffset.Sync(gommap.MS_SYNC)
		stateOffset.UnsafeUnmap()
		fd.Close()
	}()

	consumer := stream.NewConsumer(
		stream.WithEOFBehaviour(stream.EOFBehaviourPoll),
		stream.FromOffset(int64(Encoding.Uint64(stateOffset))))
	cursor := s.log.Reader()
	defer cursor.Close()

	return consumer.Consume(ctx, cursor, func(c context.Context, b stream.Batch) error {
		for idx, record := range b.Records {
			err := f(mustDecode(record))
			if err != nil {
				return err
			}
			Encoding.PutUint64(stateOffset, b.FirstOffset+uint64(idx))
		}
		return nil
	})
}
