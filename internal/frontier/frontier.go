package frontier

import (
	"context"
	"net/url"
	"sync"
)

type Task struct {
	URL *url.URL
}

type Frontier struct {
	ch       chan Task
	wg       sync.WaitGroup
	mu       sync.Mutex
	seen     map[string]bool
	max      int
	failures int
}

func NewFrontier(max int) *Frontier {
	return &Frontier{ch: make(chan Task), seen: make(map[string]bool), max: max}
}

func (f *Frontier) Enqueue(ctx context.Context, t Task) {
	u := canonical(t.URL)
	t.URL = u
	key := u.String()

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.seen[key] {
		return
	}

	// When set f.max the frontier accepts up to the f.max number of distinct urls.
	// If not set it's unlimited
	if f.max > 0 && len(f.seen) >= f.max {
		return
	}
	f.seen[key] = true

	// Increment wait group by 1, so we can track outstanding tasks that need to be processed
	f.wg.Add(1)

	// The chanel is drained by available workers. If all workers are busy and we try
	// to push to the channel, we will be blocked because there is no worker ready to receive
	//
	// The worker will also not be able to finish because the worker
	// itself calls Enqueue that is already blocked waiting to push more work to the channel
	//
	// To fix this, we use a goroutine that will add the task to the channel when a worker
	// is free to receive it
	//
	// This allows Enqueue to return which in turn allows the worker to return and be free to receive
	go f.addTaskToFrontier(ctx, t)
}

func (f *Frontier) addTaskToFrontier(ctx context.Context, t Task) {
	// If the context is cancelled stop pushing new tasks to the channel,
	// kill the goroutine so it's not parked forever and leaks
	select {
	case f.ch <- t:
	case <-ctx.Done():
		// Important: This is to balance the Add we just made. Without this
		// wg never reaches 0 and the goroutine in Watch method parks forever
		// leading to a leak
		f.Ack()
	}

}

func (f *Frontier) Ack()               { f.wg.Done() }
func (f *Frontier) Fail()              { f.wg.Done(); f.mu.Lock(); f.failures++; f.mu.Unlock() }
func (f *Frontier) Tasks() <-chan Task { return f.ch }
func (f *Frontier) Failures() int      { f.mu.Lock(); defer f.mu.Unlock(); return f.failures }

// Wait for the frontier to finish with all work then close the channel
// It's important to make sure that for every wg.Add there is a corresponding wg.Done either
// when the task is completed or when the task is cancelled
func (f *Frontier) Watch() { go func() { f.wg.Wait(); close(f.ch) }() }

func canonical(u *url.URL) *url.URL {
	c := *u
	if c.Path == "" {
		c.Path = "/"
	}
	return &c
}
