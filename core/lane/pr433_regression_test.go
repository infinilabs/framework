package lane

import (
	"testing"
	"time"
)

// Delete of an absent lane must stay idempotent WITHOUT holding the
// registry write lock — a leaked lock blocks every later lane operation
// process-wide (regression: PR #433 review, absent-name early return).
func TestDeleteAbsentLaneKeepsRegistryUsable(t *testing.T) {
	for _, mode := range []string{"never-created", "already-deleted"} {
		t.Run(mode, func(t *testing.T) {
			name := t.Name()
			if mode == "already-deleted" {
				if _, err := Create(LaneConfig{Name: name}); err != nil {
					t.Fatal(err)
				}
				if err := Delete(name, true); err != nil {
					t.Fatal(err)
				}
			}
			if err := Delete(name, true); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() { Get("unrelated-lane"); close(done) }()
			select {
			case <-done:
			case <-time.After(100 * time.Millisecond):
				// if the lock leaked anyway, release it so this failure
				// does not hang the rest of the package's tests
				regMu.Unlock()
				<-done
				t.Fatal("Delete of absent lane leaked the registry write lock; unrelated Get blocked")
			}
		})
	}
}
