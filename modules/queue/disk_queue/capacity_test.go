package queue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
)

// Capacity limits count all local queues without counting other data or requiring an existing queue directory.
func TestCheckCapacityQueueUsage(t *testing.T) {
	originalEnv := global.Env()
	originalPreventRead := preventRead
	t.Cleanup(func() {
		global.RegisterEnv(originalEnv)
		preventRead = originalPreventRead
	})

	// Each case defines filesystem inputs and the expected capacity-check result.
	//
	// Separate queue usage from partition free space so an error from the wrong check cannot satisfy a case.
	for _, tc := range []struct {
		// Scenario label shown in subtest output.
		name string
		// Bytes stored in each queue. Nil leaves the queue root absent; an empty slice creates an empty root.
		queueSizes []int
		// Partition free-space threshold in bytes; zero disables this check.
		reservedFreeBytes uint64
		// Value of preventRead before the check; true simulates a previous capacity failure.
		initialPreventRead bool
		// Expected error substring; empty means success with preventRead cleared.
		wantError string
	}{
		{name: "queue directory absent"},
		{name: "queue directory empty", queueSizes: []int{}},
		{name: "multiple queues below limit unblock writes", queueSizes: []int{40, 59}, initialPreventRead: true},
		{name: "multiple queues at limit", queueSizes: []int{40, 60}, wantError: "disk usage"},
		{name: "multiple queues above limit", queueSizes: []int{40, 61}, wantError: "disk usage"},
		{name: "free space checked without queue directory", reservedFreeBytes: ^uint64(0), wantError: "disk free space"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// GetDataDir caches its resolved path, so each case needs a fresh environment.
			testEnv := env.EmptyEnv()
			testEnv.SystemConfig.PathConfig.Data = t.TempDir()
			// Filesystem errors must fail the test instead of being swallowed by production panic recovery.
			testEnv.IsDebug = true
			global.RegisterEnv(testEnv)
			preventRead = tc.initialPreventRead
			// Non-queue data alone exceeds the configured limit in every case.
			servicePath := filepath.Join(global.Env().GetDataDir(), "easysearch", "node")
			if err := os.MkdirAll(servicePath, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(servicePath, "index.dat"), make([]byte, 1024), 0644); err != nil {
				t.Fatal(err)
			}
			if tc.queueSizes != nil {
				if err := os.MkdirAll(GetDataPath(""), 0755); err != nil {
					t.Fatal(err)
				}
			}
			for index, size := range tc.queueSizes {
				queuePath := GetDataPath([]string{"first", "second"}[index])
				if err := os.MkdirAll(queuePath, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(queuePath, "000000000.dat"), make([]byte, size), 0644); err != nil {
					t.Fatal(err)
				}
			}

			err := checkCapacity(&DiskQueueConfig{
				MaxUsedBytes:      100,
				ReservedFreeBytes: tc.reservedFreeBytes,
			})
			expectedPreventRead := tc.wantError != ""
			if (err != nil) != expectedPreventRead || (err != nil && !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("checkCapacity() error = %v, want error containing %q", err, tc.wantError)
			}
			if preventRead != expectedPreventRead {
				t.Fatalf("preventRead = %v, want %v", preventRead, expectedPreventRead)
			}
		})
	}
}
