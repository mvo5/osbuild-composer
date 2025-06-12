package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	Run = run
)

func MockIcliBinary(t *testing.T, new string) (restore func()) {
	t.Helper()

	saved := ibcliBinary

	tmpdir := t.TempDir()
	ibcliBinary = filepath.Join(tmpdir, "image-builder")
	/* #nosec G306 */
	err := os.WriteFile(ibcliBinary, []byte(new), 0755)
	require.NoError(t, err)

	return func() {
		ibcliBinary = saved
	}
}
