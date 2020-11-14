package ack

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vx-labs/mqtt-protocol/packet"
)

func TestQueue(t *testing.T) {
	t.Run("publish", func(t *testing.T) {
		t.Run("qos 0", func(t *testing.T) {
			queue := NewQueue()
			p := &packet.Publish{Header: &packet.Header{Qos: 0}, MessageId: 1}
			require.Error(t, queue.Insert("0", p, time.Time{}, func(expired bool, stored, received packet.Packet) {}))
		})
		t.Run("qos 1", func(t *testing.T) {
			p := &packet.Publish{Header: &packet.Header{Qos: 1}, MessageId: 1}
			t.Run("acked", func(t *testing.T) {
				queue := NewQueue()
				triggerd := false
				timeout := false
				require.NoError(t, queue.Insert("0", p,
					time.Time{},
					func(expired bool, store, recieved packet.Packet) {
						timeout = expired
						triggerd = true
					}))
				require.NoError(t, queue.Ack("0", &packet.PubAck{
					Header:    &packet.Header{},
					MessageId: 1,
				}))
				require.True(t, triggerd)
				require.False(t, timeout)
			})
			t.Run("timed out", func(t *testing.T) {
				queue := NewQueue()
				triggerd := false
				timeout := false
				require.NoError(t, queue.Insert("0", p,
					time.Time{},
					func(expired bool, store, received packet.Packet) {
						triggerd = true
						timeout = expired
					}))
				queue.Expire(time.Time{}.Add(1 * time.Second))
				require.Error(t, queue.Ack("0", &packet.PubAck{
					Header:    &packet.Header{},
					MessageId: 1,
				}))
				require.True(t, triggerd)
				require.True(t, timeout)
			})
		})
		t.Run("qos 2", func(t *testing.T) {
			p := &packet.Publish{Header: &packet.Header{Qos: 2}, MessageId: 1}
			t.Run("acked", func(t *testing.T) {
				queue := NewQueue()
				triggerd := false
				timeout := false
				require.NoError(t, queue.Insert("0", p,
					time.Time{},
					func(expired bool, store, recieved packet.Packet) {
						queue.Insert("0", &packet.PubRel{Header: &packet.Header{}, MessageId: 1},
							time.Time{},
							func(expired bool, stored, received packet.Packet) {
								triggerd = true
								timeout = expired
							})
					}))
				require.NoError(t, queue.Ack("0", &packet.PubRec{
					Header:    &packet.Header{},
					MessageId: 1,
				}))
				require.NoError(t, queue.Ack("0", &packet.PubComp{
					Header:    &packet.Header{},
					MessageId: 1,
				}))
				require.True(t, triggerd)
				require.False(t, timeout)
			})
		})
	})
}
