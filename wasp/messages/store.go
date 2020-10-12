package messages

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"path"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
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
	ErrIndexOutdated         = errors.New("index outdated")
	messageBucketName        = []byte("messages")
	configBucketName         = []byte("config")
	maxMessageCount   uint64 = 5000
)

type Log interface {
	io.Closer
	Append(payload []*StoredMessage) error
	Consume(ctx context.Context, consumerName string, f func(string, *packet.Publish) error) error
}

type messageLog struct {
	notifications     map[string]chan struct{}
	notificationsLock sync.RWMutex
	conn              *bolt.DB
	options           Options
}

const (
	// Permissions to use on the db file. This is only used if the
	// database file does not exist and needs to be created.
	dbFileMode = 0600
)

type Options struct {
	// Path is the file path to the BoltDB to use
	Path string

	// BoltOptions contains any specific BoltDB options you might
	// want to specify [e.g. open timeout]
	BoltOptions *bolt.Options

	// NoSync causes the database to skip fsync calls after each
	// write to the log. This is unsafe, so it should be used
	// with caution.
	NoSync bool
}

func mustEncode(p *StoredMessage) []byte {
	buf, err := proto.Marshal(p)
	if err != nil {
		panic(err)
	}
	return buf
}
func mustDecode(b []byte) *StoredMessage {
	p := &StoredMessage{}
	err := proto.Unmarshal(b, p)
	if err != nil {
		panic(err)
	}
	return p
}

func New(options Options) (Log, error) {
	handle, err := bolt.Open(path.Join(options.Path, "messages"), dbFileMode, options.BoltOptions)
	if err != nil {
		return nil, err
	}
	handle.NoSync = options.NoSync

	// Create the new store
	store := &messageLog{
		conn:          handle,
		options:       options,
		notifications: make(map[string]chan struct{}),
	}
	handle.Update(func(tx *bolt.Tx) error {
		messages, err := tx.CreateBucketIfNotExists(messageBucketName)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(configBucketName)
		if err != nil {
			return err
		}
		store.trim(messages)
		return nil
	})
	return store, nil
}

func (l *messageLog) Close() error {
	return l.conn.Close()
}

func (b *messageLog) notify() {
	b.notificationsLock.RLock()
	defer b.notificationsLock.RUnlock()
	for _, ch := range b.notifications {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
func (b *messageLog) subscribe(id string, ch chan struct{}) {
	b.notificationsLock.Lock()
	defer b.notificationsLock.Unlock()
	if _, ok := b.notifications[id]; ok {
		close(b.notifications[id])
		delete(b.notifications, id)
	}
	b.notifications[id] = ch
}
func (b *messageLog) unsubscribe(id string) {
	b.notificationsLock.Lock()
	defer b.notificationsLock.Unlock()
	if _, ok := b.notifications[id]; ok {
		close(b.notifications[id])
		delete(b.notifications, id)
	}
}

func (b *messageLog) Append(payload []*StoredMessage) error {
	tx, err := b.conn.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bucket := tx.Bucket(messageBucketName)
	if bucket == nil {
		return ErrBucketNotFound
	}
	for idx := range payload {
		offset, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		err = bucket.Put(uint64ToBytes(offset), mustEncode(payload[idx]))
		if err != nil {
			return err
		}
	}
	b.trim(bucket)
	err = tx.Commit()
	if err == nil {
		b.notify()
	}
	return err
}

func (b *messageLog) trim(bucket *bolt.Bucket) {
	seq := bucket.Sequence()
	if seq > maxMessageCount {
		cursor := bucket.Cursor()
		cut := bucket.Sequence() - maxMessageCount
		for itemKey, _ := cursor.First(); itemKey != nil && bytesToUint64(itemKey) < cut; itemKey, _ = cursor.Next() {
			err := bucket.Delete(itemKey)
			if err != nil {
				break
			}
		}
	}
}
func (b *messageLog) advanceOffset(consumer []byte, offset uint64) error {
	return b.conn.Update(func(tx *bolt.Tx) error {
		config := tx.Bucket(configBucketName)
		return config.Put(consumer, uint64ToBytes(offset))
	})
}
func (b *messageLog) get(offset uint64, buff []*StoredMessage) (int, uint64, error) {
	tx, err := b.conn.Begin(false)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	bucket := tx.Bucket(messageBucketName)
	if bucket == nil {
		return 0, 0, ErrBucketNotFound
	}
	idx := 0
	cursor := bucket.Cursor()
	count := len(buff)
	for itemKey, itemValue := cursor.Seek(uint64ToBytes(offset)); itemKey != nil && idx < count; itemKey, itemValue = cursor.Next() {
		offset = bytesToUint64(itemKey)
		buff[idx] = mustDecode(itemValue)
		idx++
	}
	if idx == 0 {
		return idx, offset, nil
	}
	return idx, offset + 1, nil
}

func (b *messageLog) consumerOffset(consumer []byte) uint64 {
	var offset uint64
	err := b.conn.View(func(tx *bolt.Tx) error {
		config := tx.Bucket(configBucketName)
		offset = bytesToUint64(config.Get(consumer))
		return nil
	})
	if err != nil {
		return 0
	}
	return offset
}
func (b *messageLog) Consume(ctx context.Context, consumerName string, f func(string, *packet.Publish) error) error {
	id := uuid.New().String()
	buf := make([]*StoredMessage, 10)
	notifications := make(chan struct{}, 1)
	consumer := []byte(consumerName)
	notifications <- struct{}{}
	go func() {
		<-ctx.Done()
		b.unsubscribe(id)
	}()
	var lastSeen uint64 = b.consumerOffset(consumer)
	var err error
	b.subscribe(id, notifications)
	for range notifications {
		for {
			count, next, err := b.get(lastSeen, buf)
			if err != nil {
				return err
			}
			for _, p := range buf[0:count] {
				err = f(p.Sender, p.Publish)
				if err != nil {
					log.Print(err)
					select {
					case <-time.After(5 * time.Second):
					case <-ctx.Done():
						return nil
					}
					break
				}
				lastSeen++
			}
			lastSeen = next
			b.advanceOffset(consumer, lastSeen)
			if count < len(buf) {
				break
			}
		}
	}
	return err
}
