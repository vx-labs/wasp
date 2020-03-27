package rpc

import (
	"context"

	"github.com/vx-labs/wasp/wasp/api"
)

func (t *Transport) GetMembers(ctx context.Context, in *api.GetMembersRequest) (*api.GetMembersResponse, error) {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	members := []*api.Member{
		{ID: t.nodeID, Address: t.nodeAddress, IsLeader: t.raft.IsLeader(t.nodeID), IsAlive: true},
	}

	for id, peer := range t.peers {
		members = append(members, &api.Member{
			ID:       id,
			Address:  peer.Conn.Target(),
			IsLeader: t.raft.IsLeader(id),
			IsAlive:  peer.Enabled,
		})
	}
	return &api.GetMembersResponse{Members: members}, nil
}
