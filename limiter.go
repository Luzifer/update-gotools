package main

import "sync"

type limiter struct {
	l chan struct{}
	w sync.WaitGroup
}

func newLimiter(max int) *limiter {
	return &limiter{
		l: make(chan struct{}, max),
	}
}

func (l *limiter) Add() {
	l.l <- struct{}{}
	l.w.Add(1)
}

func (l *limiter) Done() {
	<-l.l
	l.w.Done()
}

func (l *limiter) Wait() {
	l.w.Wait()
}
