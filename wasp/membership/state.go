package membership

func (l *Gossip) NotifyMsg(b []byte) {
}
func (l *Gossip) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}
func (l *Gossip) LocalState(join bool) []byte {
	return nil
}

func (m *Gossip) MergeRemoteState(buf []byte, join bool) {}
