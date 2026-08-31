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
	failures int
}

func NewFrontier() *Frontier { return &Frontier{ch: make(chan Task), seen: make(map[string]bool)} }

func (f *Frontier) Enqueue(t Task) {
	key := t.URL.String()
	f.mu.Lock()
	if f.seen[key] {
		f.mu.Unlock()
		return
	}
	f.seen[key] = true
	f.mu.Unlock()
	f.wg.Add(1)
	go func() { f.ch <- t }()
}

func (f *Frontier) Ack()               { f.wg.Done() }
func (f *Frontier) Fail()              { f.wg.Done(); f.mu.Lock(); f.failures++; f.mu.Unlock() }
func (f *Frontier) Tasks() <-chan Task { return f.ch }
func (f *Frontier) Failures() int      { return f.failures }
func (f *Frontier) Watch()             { go func() { f.wg.Wait(); close(f.ch) }() }
