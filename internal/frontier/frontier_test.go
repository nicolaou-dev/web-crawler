package frontier

import (
	"fmt"
	"net/url"
	"sync"
	"testing"
)

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// consume drains the frontier, calling handle for each task, and blocks until
// all work is acked.
func consume(f *Frontier, handle func(Task)) {
	go f.Watch()
	for t := range f.Tasks() {
		handle(t)
	}
}

// drain consumes the frontier, returning the number of distinct tasks processed
// and how many times each was processed.
func drain(f *Frontier) (int, map[string]int) {
	counts := map[string]int{}
	consume(f, func(t Task) {
		counts[t.URL.String()]++
		f.Ack()
	})
	return len(counts), counts
}

// Enqueuing the same URL concurrently accepts and processes it only once.
func TestConcurrentEnqueueDedups(t *testing.T) {
	f := NewFrontier(0)
	u := mustURL("http://example.com/a")

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() { f.Enqueue(Task{URL: u}) })
	}
	wg.Wait()

	if got, _ := drain(f); got != 1 {
		t.Errorf("processed %d tasks, want 1", got)
	}
}

// Enqueuing distinct URLs concurrently processes all of them.
func TestConcurrentEnqueueLosesNothing(t *testing.T) {
	f := NewFrontier(0)

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() { f.Enqueue(Task{URL: mustURL(fmt.Sprintf("http://example.com/%d", i))}) })
	}
	wg.Wait()

	if got, _ := drain(f); got != 100 {
		t.Errorf("processed %d tasks, want 100", got)
	}
}

// A URL that has already been enqueued is not enqueued again.
func TestDuplicateIgnored(t *testing.T) {
	f := NewFrontier(0)
	u := mustURL("http://example.com/a")

	f.Enqueue(Task{URL: u})
	f.Enqueue(Task{URL: u})

	if got, _ := drain(f); got != 1 {
		t.Errorf("processed %d tasks, want 1", got)
	}
}

// Tasks enqueued while other tasks are being processed are handled, and the
// frontier still terminates.
func TestEnqueueDuringProcessingTerminates(t *testing.T) {
	f := NewFrontier(0)
	root := mustURL("http://example.com/root")
	f.Enqueue(Task{URL: root})

	processed := 0
	consume(f, func(t Task) {
		if t.URL.String() == root.String() {
			for j := range 3 {
				f.Enqueue(Task{URL: mustURL(fmt.Sprintf("http://example.com/child-%d", j))})
			}
		}
		processed++
		f.Ack()
	})

	if processed != 4 {
		t.Errorf("processed %d tasks, want 4 (root + 3 children)", processed)
	}
}

func TestMaxBudget(t *testing.T) {
	f := NewFrontier(50)

	for i := range 100 {
		u := mustURL(fmt.Sprintf("http://example.com/page/%d", i))
		f.Enqueue(Task{URL: u})
	}

	if got, _ := drain(f); got != 50 {
		t.Errorf("processed %d tasks, want 50", got)
	}

}

// Fail counts the failure and still lets the frontier terminate.
func TestFailCountsAndTerminates(t *testing.T) {
	f := NewFrontier(0)
	f.Enqueue(Task{URL: mustURL("http://example.com/a")})
	f.Enqueue(Task{URL: mustURL("http://example.com/b")})

	consume(f, func(t Task) { f.Fail() })

	if got := f.Failures(); got != 2 {
		t.Errorf("failures = %d, want 2", got)
	}
}

// A mix of Ack and Fail counts only the failures and still terminates.
func TestMixedAckFail(t *testing.T) {
	f := NewFrontier(0)
	f.Enqueue(Task{URL: mustURL("http://example.com/ok")})
	f.Enqueue(Task{URL: mustURL("http://example.com/bad")})

	consume(f, func(t Task) {
		if t.URL.Path == "/bad" {
			f.Fail()
			return
		}
		f.Ack()
	})

	if got := f.Failures(); got != 1 {
		t.Errorf("failures = %d, want 1", got)
	}
}
