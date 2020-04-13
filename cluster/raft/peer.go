package raft
import (
	"fmt"

	"go.uber.org/zap/zapcore"
)

// Peer represents a remote raft peer
type Peer struct {
	ID      uint64
	Address string
}

func (p Peer) MarshalLogObject(e zapcore.ObjectEncoder) error {
	e.AddString("hex_peer_id", fmt.Sprintf("%x", p.ID))
	e.AddString("peer_address", p.Address)

	return nil
}

type Peers []Peer

func (p Peers) MarshalLogArray(e zapcore.ArrayEncoder) error {

	for idx := range p {
		if err := e.AppendObject(p[idx]); err != nil {
			return err
		}
	}

	return nil
}
