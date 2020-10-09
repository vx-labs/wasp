package membership

func (p *pool) NotifyMsg(b []byte) {
}
func (p *pool) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}
func (p *pool) LocalState(join bool) []byte {
	return nil
}

func (p *pool) MergeRemoteState(buf []byte, join bool) {}
