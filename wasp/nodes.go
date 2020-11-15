package wasp

import (
	"time"

	"github.com/vx-labs/wasp/wasp/distributed"
)

type NodeMemberManager interface {
	NotifyGossipJoin(id uint64)
	NotifyGossipLeave(id uint64)
}

type nodeMemberManager struct {
	id    uint64
	log   messageLog
	state distributed.State
}

func NewNodeMemberManager(id uint64, log messageLog, state distributed.State) NodeMemberManager {
	return &nodeMemberManager{
		id:    id,
		log:   log,
		state: state,
	}
}

func (n *nodeMemberManager) NotifyGossipJoin(id uint64) {}
func (n *nodeMemberManager) NotifyGossipLeave(id uint64) {
	n.state.Subscriptions().DeletePeer(id)
	sessions := n.state.SessionMetadatas().ByPeer(id)
	for _, session := range sessions {
		lwt := session.LWT
		if lwt != nil {
			n.log.Append(lwt)
		}
	}
	go func() {
		<-time.After(3 * time.Second)
		n.state.SessionMetadatas().DeletePeer(id)
	}()
}
