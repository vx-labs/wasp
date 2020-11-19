package expiration

import (
	"time"

	"github.com/zond/gotomic"
)

type List interface {
	Insert(id gotomic.Hashable, deadline time.Time)
	Delete(id gotomic.Hashable, deadline time.Time) bool
	Update(id gotomic.Hashable, old time.Time, new time.Time)
	Expire(now time.Time) []interface{}
	Reset()
}

func NewList() List {
	return newSkipList()
}
