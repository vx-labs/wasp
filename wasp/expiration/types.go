package expiration

import (
	"time"
)

type List interface {
	Insert(id interface{}, deadline time.Time)
	Delete(id interface{}, deadline time.Time) bool
	Update(id interface{}, old time.Time, new time.Time)
	Expire(now time.Time) []interface{}
	Reset()
}

func NewList() List {
	return newPQList()
}
