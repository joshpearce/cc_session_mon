package session

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestGetSessions_ConcurrentWithRefresh reproduces the render-vs-watcher data
// race: the watcher goroutine recomputes a session's metrics in place while a
// reader (the TUI render path) iterates the sessions GetSessions handed it. With
// the snapshot copy in GetSessions the reader works on its own value, so
// `go test -race` must stay clean.
//
// The loops are bounded by a fixed iteration count (no wall-clock, no busy-wait)
// so the test finishes in well under a second even under -race.
func TestGetSessions_ConcurrentWithRefresh(t *testing.T) {
	const iterations = 2000

	dir := t.TempDir()
	main := filepath.Join(dir, "sess.jsonl")
	if err := os.WriteFile(main, []byte(usageLine("2026-06-17T10:00:00Z", 100, 100)), 0o600); err != nil {
		t.Fatal(err)
	}

	w := &Watcher{
		sessions:    map[string]*Session{},
		subagentMap: map[string]string{},
		offsets:     map[string]int64{},
		lineNumbers: map[string]int{},
		originMap:   map[string]string{},
	}
	sess := &Session{ID: "sess", FilePath: main, IsActive: true}
	refreshSessionMetrics(main, sess)
	w.sessions[main] = sess
	w.invalidateSortedCache()

	var wg sync.WaitGroup

	// Writer: the watcher goroutine mutating metrics in place under the lock, as
	// handleFileUpdate does on every fsnotify event.
	wg.Go(func() {
		for range iterations {
			w.mu.Lock()
			refreshSessionMetrics(main, sess)
			w.invalidateSortedCache()
			w.mu.Unlock()
		}
	})

	// Reader: the render path reading the snapshot's metric fields.
	wg.Go(func() {
		for range iterations {
			for _, s := range w.GetSessions() {
				_ = s.Burn.TokensPerMinute
				_ = s.BurnRecent.TokensPerMinute
				_ = s.AgentStats.MaxConcurrent
				_ = s.AgentStats.ActiveWithin(time.Now(), RecentWindow)
				_ = len(s.Commands)
			}
		}
	})

	wg.Wait()
}
