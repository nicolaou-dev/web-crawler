package frontier

import (
	"fmt"
	"net/url"
	"sync"
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package if any test leaks a goroutine — e.g. a coordinator
// that never exits because the frontier was neither drained nor closed.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// Enqueuing the same URL concurrently accepts and hands it out only once.
func TestConcurrentEnqueueDedups(t *testing.T) {
	u := mustURL("http://example.com/a")
	f := NewInMemory(u, 0)
	defer f.Close()

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() { f.Enqueue(Task{URL: u}) })
	}
	wg.Wait()

	got := 0
	for {
		task, ok := f.Next()
		if !ok {
			break
		}
		got++
		f.Ack(task)
	}

	if got != 1 {
		t.Errorf("processed %d tasks, want 1", got)
	}
}

// Enqueuing distinct URLs concurrently loses none of them (seed + 100).
func TestConcurrentEnqueueLosesNothing(t *testing.T) {
	f := NewInMemory(mustURL("http://example.com"), 0)
	defer f.Close()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() { f.Enqueue(Task{URL: mustURL(fmt.Sprintf("http://example.com/%d", i))}) })
	}
	wg.Wait()

	got := 0
	for {
		task, ok := f.Next()
		if !ok {
			break
		}
		got++
		f.Ack(task)
	}

	if got != 101 {
		t.Errorf("processed %d tasks, want 101 (seed + 100)", got)
	}
}

// A URL that has already been enqueued is not enqueued again.
func TestDuplicateIgnored(t *testing.T) {
	u := mustURL("http://example.com/a")
	f := NewInMemory(u, 0)
	defer f.Close()

	f.Enqueue(Task{URL: u})

	got := 0
	for {
		task, ok := f.Next()
		if !ok {
			break
		}
		got++
		f.Ack(task)
	}

	if got != 1 {
		t.Errorf("processed %d tasks, want 1", got)
	}
}

// Tasks enqueued while another task is being processed are handled, and the
// frontier still terminates.
func TestEnqueueDuringProcessingTerminates(t *testing.T) {
	root := mustURL("http://example.com/root")
	f := NewInMemory(root, 0)
	defer f.Close()

	processed := 0
	for {
		task, ok := f.Next()
		if !ok {
			break
		}
		if task.URL.String() == root.String() {
			for j := range 3 {
				f.Enqueue(Task{URL: mustURL(fmt.Sprintf("http://example.com/child-%d", j))})
			}
		}
		processed++
		f.Ack(task)
	}

	if processed != 4 {
		t.Errorf("processed %d tasks, want 4 (root + 3 children)", processed)
	}
}

// The max budget caps the number of distinct URLs, counting the seed.
func TestMaxBudget(t *testing.T) {
	f := NewInMemory(mustURL("http://example.com/"), 50)
	defer f.Close()

	for i := range 100 {
		f.Enqueue(Task{URL: mustURL(fmt.Sprintf("http://example.com/page/%d", i))})
	}

	got := 0
	for {
		task, ok := f.Next()
		if !ok {
			break
		}
		got++
		f.Ack(task)
	}

	if got != 50 {
		t.Errorf("processed %d tasks, want 50 (cap includes seed)", got)
	}
}

// Fail completes a task just like Ack, so the frontier still terminates.
func TestFailTerminates(t *testing.T) {
	f := NewInMemory(mustURL("http://example.com/a"), 0)
	defer f.Close()

	seed, ok := f.Next()
	if !ok {
		t.Fatal("expected the seed")
	}
	f.Fail(seed)

	if _, ok := f.Next(); ok {
		t.Error("frontier did not terminate after Fail")
	}
}

// URLs are canonicalized before dedup, and the caller's URL is not mutated.
func TestEnqueueCanonicalizesDedup(t *testing.T) {
	u1 := mustURL("https://books.toscrape.com")  // empty path
	u2 := mustURL("https://books.toscrape.com/") // canonical form of u1
	f := NewInMemory(u1, 0)
	defer f.Close()

	f.Enqueue(Task{URL: u2})

	counts := map[string]int{}
	for {
		task, ok := f.Next()
		if !ok {
			break
		}
		counts[task.URL.String()]++
		f.Ack(task)
	}

	if len(counts) != 1 {
		t.Errorf("processed %d tasks, want 1 (canonical dedup)", len(counts))
	}
	if counts["https://books.toscrape.com/"] != 1 {
		t.Errorf("counts = %v, want single https://books.toscrape.com/", counts)
	}
	if u1.Path != "" {
		t.Errorf("caller URL mutated: path=%q", u1.Path)
	}
}

// Close stops the coordinator goroutine even when the frontier is never drained
// — the shutdown path the draining tests above don't exercise.
func TestCloseStopsCoordinator(t *testing.T) {
	defer goleak.VerifyNone(t)

	f := NewInMemory(mustURL("http://example.com/"), 0)
	f.Close()
}
