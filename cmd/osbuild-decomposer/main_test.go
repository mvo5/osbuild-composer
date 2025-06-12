package main_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-decomposer"
)

func runTestServer(t *testing.T) (baseURL, buildBaseDir string) {
	t.Helper()

	baseOutputDir := t.TempDir()
	socketPath := filepath.Join(buildBaseDir, "osbuild-decomposer-httpd.sock")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	args := []string{
		"--base-output-dir", baseOutputDir,
		"--socket-path", socketPath,
	}
	go func() {
		err := main.Run(ctx, args, os.Getenv)
		require.NoError(t, err, "running osbuild-decomposer")
	}()

	// wait for the socket to be created
	for {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return socketPath, baseOutputDir
}
