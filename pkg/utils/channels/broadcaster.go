package channels

import (
	"context"
	"sync"
)

type Broadcaster[T any] struct {
	lock        *sync.RWMutex
	subscribers map[*Subscriber[T]]struct{}
	value       T
}

func NewBroadcaster[T any](value T) *Broadcaster[T] {
	lock := new(sync.RWMutex)
	return &Broadcaster[T]{
		lock:        lock,
		subscribers: make(map[*Subscriber[T]]struct{}),
		value:       value,
	}
}

func (b *Broadcaster[T]) Publish(value T) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.value = value
	for s := range b.subscribers {
		s.in <- value
	}
}

func (b *Broadcaster[T]) Value() T {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.value
}

type Subscriber[T any] struct {
	ctx    context.Context
	source *Broadcaster[T]
	in     chan T
	out    chan chan T
}

func NewSubscriber[T any](ctx context.Context, source *Broadcaster[T]) *Subscriber[T] {
	source.lock.Lock()
	defer source.lock.Unlock()

	s := &Subscriber[T]{
		ctx:    ctx,
		source: source,
		in:     make(chan T),
		out:    make(chan chan T),
	}
	source.subscribers[s] = struct{}{}

	go s.subscribe()

	return s
}

func (s *Subscriber[T]) subscribe() {
	defer func() {
		s.source.lock.Lock()
		defer s.source.lock.Unlock()

		delete(s.source.subscribers, s)
	}()

	value := s.source.Value()

	var ch chan T
	for {
		select {
		case <-s.ctx.Done():
			return
		case ch = <-s.out:
		}

		stop := false
		for !stop {
			select {
			case <-s.ctx.Done():
				stop = true
			case ch <- value:
				stop = true

			case next := <-s.in:
				value = next
			}
		}

		close(ch)

		select {
		case <-s.ctx.Done():
			return
		case next := <-s.in:
			value = next
		}
	}
}

func (s *Subscriber[T]) Wait() <-chan T {
	ch := make(chan T)
	select {
	case s.out <- ch:
		return ch
	case <-s.ctx.Done():
		return nil
	}
}
