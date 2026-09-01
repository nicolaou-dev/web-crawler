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
	// Close the out channel when the frontier is done
	// so workers can no longer pull work
	defer close(m.out)

	// TODO: Bound seen. If it's a massive crawl it can OOM
	seen := map[string]bool{seed.URL.String(): true}
	// TODO: Bound the queue. Since this is in memory, we can
	// only skip additional work. In a durable frontier
	// tasks are persisted
	queue := []Task{seed}

	// Tracks in flight async work
	inFlight := 0

	for {
		// When work is handed out to the workers the queue could
		// temporarily be empty, therefore we need to make sure
		// that there is no more in flight work before we close
		// the loop
		if len(queue) == 0 && inFlight == 0 {
			return
		}

		// Initialize out as nil on every iteration
		var out chan Task
		var next Task

		// If we have items in the queue assign the frontier's out
		// channel (that hands work to the workers) to out, and
		// get the next task to hand out
		if len(queue) > 0 {
			out, next = m.out, queue[0]
		}

		select {
		// This case is always live. The frontier is always ready
		// to receive tasks, so workers are never blocked trying to
		// enqueue work
		case t := <-m.incoming:
			key := t.URL.String()

			// Check that we haven't see this task before or that
			// we haven't reached the max number of tasks we can process
			if seen[key] || (m.max > 0 && len(seen) >= m.max) {
				continue
			}
			seen[key] = true

			queue = append(queue, t)
		// If out is nil this case is skipped, so no work is handed out
		case out <- next:
			// Remove the task from the queue and increment the in flight
			// counter.
			// This is important so the frontier doesn't exit while async work
			// is still in progress.
			queue = queue[1:]
			inFlight++
		case <-m.acked:
			// Once the task is done, decrement in flight work counter
			inFlight--
		// Manually close the loop. This is called when context is cancelled
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

func (m *Memory) Fail(_ Task) {
	// TODO: Retry transient errors, move to DLQ
	select {
	case m.acked <- struct{}{}:
	case <-m.done:
	}
}

func (m *Memory) Close() {
	m.closeOnce.Do(func() { close(m.done) })
}
