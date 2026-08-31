package frontier

import (
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

func (f *Frontier) Enqueue(t Task) {
	defer f.mu.Unlock()
	key := t.URL.String()
	f.mu.Lock()
	if f.seen[key] {
		return
	}
	// When set f.max the frontier accepts up to the f.max number of distinct urls.
	// If not set it's unlimited
	if f.max > 0 && len(f.seen) >= f.max {
		return
	}
	f.seen[key] = true
	f.wg.Add(1)

	go func() { f.ch <- t }()
}

func (f *Frontier) Ack()               { f.wg.Done() }
func (f *Frontier) Fail()              { f.wg.Done(); f.mu.Lock(); f.failures++; f.mu.Unlock() }
func (f *Frontier) Tasks() <-chan Task { return f.ch }
func (f *Frontier) Failures() int      { return f.failures }
func (f *Frontier) Watch()             { go func() { f.wg.Wait(); close(f.ch) }() }
