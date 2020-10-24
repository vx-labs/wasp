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
	Consume(ctx context.Context, consumerName string, f func(uint64, *packet.Publish) error) error
	Stream(ctx context.Context, consumer stream.Consumer, f func(*packet.Publish) error) error
}

type store struct {
	datadir string
	log     commitlog.CommitLog
}

func New(datadir string) (Log, error) {
	log, err := commitlog.Open(path.Join(datadir, "log"), 500)
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

func (s *store) Stream(ctx context.Context, consumer stream.Consumer, f func(*packet.Publish) error) error {
	reader := s.log.Reader()
	reader.Seek(0, io.SeekEnd)
	return consumer.Consume(ctx, reader, func(c context.Context, b stream.Batch) error {
		for _, record := range b.Records {
			err := f(mustDecode(record))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *store) maybeTruncate(currentOffset uint64) {
	if currentOffset > 1500 && currentOffset%1000 == 0 {
		s.log.TruncateBefore(currentOffset - 300)
	}
}
func (s *store) Consume(ctx context.Context, consumerName string, f func(uint64, *packet.Publish) error) error {

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

	offset := Encoding.Uint64(stateOffset)
	consumer := stream.NewConsumer(
		stream.WithEOFBehaviour(stream.EOFBehaviourPoll),
		stream.FromOffset(int64(offset)))
	cursor := s.log.Reader()
	s.maybeTruncate(offset)
	return consumer.Consume(ctx, cursor, func(c context.Context, b stream.Batch) error {
		for idx, record := range b.Records {
			newOffset := b.FirstOffset + uint64(idx)

			err := f(newOffset, mustDecode(record))
			if err != nil {
				return err
			}
			offset = newOffset
			Encoding.PutUint64(stateOffset, offset)
			s.maybeTruncate(offset)
		}

		return nil
	})
}
