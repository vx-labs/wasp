package wasp

import (
	"sort"
	"sync"
)

type midPool interface {
	Get() int32
	Put(int32)
}

type interval struct {
	from int32
	to   int32
}

type simpleMidPool struct {
	mtx       sync.Mutex
	min       int32
	max       int32
	intervals []interval
}

func newMIDPool(min, max int32) midPool {
	return &simpleMidPool{
		min: min,
		max: max,
	}
}

func (m *simpleMidPool) Get() int32 {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if len(m.intervals) == 0 {
		m.intervals = []interval{
			{from: m.min, to: m.max},
		}
		return m.min
	}
	if m.intervals[0].from == m.max {
		return -1
	}
	m.intervals[0].from++
	v := m.intervals[0].from
	if m.intervals[0].from >= m.intervals[0].to {
		m.intervals = m.intervals[1:]
	}
	return v
}
func (m *simpleMidPool) Put(mid int32) {
	if mid < m.min || mid > m.max {
		return
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()

	idx := sort.Search(len(m.intervals), func(i int) bool {
		return m.intervals[i].from >= mid
	})
	if idx < len(m.intervals) && (m.intervals[idx].from < mid && m.intervals[idx].to >= mid) {
		return
	}

	if idx == len(m.intervals) {
		if m.intervals[idx-1].from < mid && m.intervals[idx-1].to >= mid {
			return
		}
		if m.intervals[idx-1].to == mid-1 {
			m.intervals[idx-1].to++
		} else {
			if mid > m.intervals[idx-1].to {
				m.intervals = append(m.intervals, interval{from: mid - 1, to: mid})
			} else {
				m.intervals = append(m.intervals[:idx-1], interval{from: mid - 1, to: mid}, m.intervals[idx-1])
			}
		}
	} else if idx > 0 && idx != len(m.intervals) {
		if m.intervals[idx].to == mid-1 {
			m.intervals[idx].to++
		} else if m.intervals[idx-1].to == mid-1 {
			m.intervals[idx-1].to++
			if m.intervals[idx-1].to == m.intervals[idx].from {
				m.intervals[idx].from = m.intervals[idx-1].from
				m.intervals = append(m.intervals[:idx-1], m.intervals[idx:]...)
			}
		} else {
			m.intervals = append(m.intervals[:idx], append([]interval{{from: mid - 1, to: mid}}, m.intervals[idx:]...)...)
		}
	} else {
		if m.intervals[0].from == mid {
			m.intervals[idx].from--
		} else {
			m.intervals = append([]interval{{from: mid - 1, to: mid}}, m.intervals...)
		}
	}
}
