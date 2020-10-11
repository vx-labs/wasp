package raft

import (
	"context"
	"errors"
)

type leaderState struct {
	f        LeaderFunc
	funcDone chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
}

func newLeaderState(f LeaderFunc) *leaderState {
	return &leaderState{
		f: f,
	}
}

func (l *leaderState) Start(ctx context.Context) error {
	if l.f == nil {
		return nil
	}
	if l.funcDone != nil {
		return errors.New("leader func is already running")
	}
	l.ctx, l.cancel = context.WithCancel(ctx)
	ch := make(chan struct{})
	l.funcDone = ch
	go func() {
		defer close(ch)
		l.f(l.ctx)
	}()
	return nil
}

func (l *leaderState) Cancel(ctx context.Context) error {
	if l.f == nil {
		return nil
	}
	defer func() {
		l.funcDone = nil
		l.ctx = nil
		l.cancel = nil
	}()
	if l.funcDone == nil {
		return errors.New("leader func is not running")
	}
	l.cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.funcDone:
		return nil
	}
}
