package epoll

type db struct {
	conns map[int]*ClientConn
}

func (d *db) Len() int {
	return len(d.conns)
}

func (d *db) Get(fd int) (*ClientConn, bool) {
	v, ok := d.conns[fd]
	return v, ok
}

func (d *db) Insert(conn *ClientConn) {
	d.conns[conn.FD] = conn
}
func (d *db) Update(conn *ClientConn) {
	d.conns[conn.FD].Deadline = conn.Deadline
}

func (d *db) Delete(fd int) {
	delete(d.conns, fd)
}
func (d *db) Range(f func(*ClientConn) bool) {
	for _, c := range d.conns {
		if f(c) {
			return
		}
	}
}
