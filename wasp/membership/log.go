package membership

import (
	"github.com/hashicorp/memberlist"
)

// NotifyJoin is called if a peer joins the cluster.
func (b *Gossip) NotifyJoin(n *memberlist.Node) {
	//b.logger.Debug("node joined", zap.String("new_node_id", n.Name))
}

// NotifyLeave is called if a peer leaves the cluster.
func (b *Gossip) NotifyLeave(n *memberlist.Node) {
	//b.logger.Debug("node left", zap.String("left_node_id", n.Name))
}

// NotifyUpdate is called if a cluster peer gets updated.
func (b *Gossip) NotifyUpdate(n *memberlist.Node) {
	//b.logger.Debug("node updated", zap.String("updated_node_id", n.Name))
}
