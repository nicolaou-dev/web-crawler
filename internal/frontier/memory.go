package frontier

import (
	"net/url"
	"sync"
)

var _ Frontier = (*Memory)(nil)

type Memory struct {
	incoming chan Task
	out      chan Task
	acked    chan struct{}
	done     chan struct{}
	max      int

	closeOnce sync.Once
}

func NewInMemory(seed *url.URL, max int) *Memory {
	m := &Memory{
		incoming: make(chan Task),
		out:      make(chan Task),
		acked:    make(chan struct{}),
		done:     make(chan struct{}),
		max:      max,
	}
	go m.coordinate(Task{URL: Canonical(seed)})
	return m
}

func (m *Memory) coordinate(seed Task) {
	defer close(m.out)

	seen := map[string]bool{seed.URL.String(): true}
	queue := []Task{seed}
	inFlight := 0

	for {
		if len(queue) == 0 && inFlight == 0 {
			return
		}

		var out chan Task
		var next Task

		if len(queue) > 0 {
			out, next = m.out, queue[0]
		}

		select {
		case t := <-m.incoming:
			key := t.URL.String()

			if seen[key] || (m.max > 0 && len(seen) >= m.max) {
				continue
			}
			seen[key] = true
			queue = append(queue, t)
		case out <- next:
			queue = queue[1:]
			inFlight++
		case <-m.acked:
			inFlight--
		case <-m.done:
			return
		}
	}
}

func (m *Memory) Enqueue(t Task) {
	t.URL = Canonical(t.URL)
	select {
	case m.incoming <- t:
	case <-m.done:
	}

}

func (m *Memory) Next() (t Task, ok bool) {
	select {
	case t, ok = <-m.out:
		return t, ok
	case <-m.done:
		return
	}
}

func (m *Memory) Ack(_ Task) {
	select {
	case m.acked <- struct{}{}:
	case <-m.done:
	}
}

func (m *Memory) Fail(t Task) {
	// TODO: Retry transient errors, move to DLQ
	select {
	case m.acked <- struct{}{}:
	case <-m.done:
	}
}

func (m *Memory) Close() {
	m.closeOnce.Do(func() { close(m.done) })
}
