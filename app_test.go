package framework

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetServiceWorkingDirectoryUsesExecutableDir(t *testing.T) {
	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable path: %v", err)
	}

	got := getServiceWorkingDirectory()
	want := filepath.Dir(executablePath)
	if got != want {
		t.Fatalf("expected service working directory %q, got %q", want, got)
	}
}
