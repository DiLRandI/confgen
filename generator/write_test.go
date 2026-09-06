package generator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenameFailureRollback(t *testing.T) {
	for _, rollbackFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "backup retained"}[rollbackFails], func(t *testing.T) {
			dir := t.TempDir()
			a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
			for _, p := range []string{a, b} {
				require.NoError(t, os.WriteFile(p, []byte("original"), 0o600))
			}
			calls := 0
			failure := errors.New("injected rename failure")
			err := writeFiles(map[string][]byte{a: []byte("new"), b: []byte("new")}, func(from, to string) error {
				calls++
				if calls == 2 || rollbackFails && calls == 3 {
					return failure
				}
				return os.Rename(from, to)
			})
			require.ErrorIs(t, err, failure, err)
			data, _ := os.ReadFile(b)
			require.Equal(t, "original", string(data), "failed destination modified")
			if !rollbackFails {
				data, _ := os.ReadFile(a)
				require.Equal(t, "original", string(data), "first output not restored")
				return
			}
			entries, _ := os.ReadDir(dir)
			found := false
			for _, entry := range entries {
				if entry.Name() == "a" || entry.Name() == "b" {
					continue
				}
				data, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
				if string(data) == "original" {
					found = true
				}
			}
			require.True(t, found, "rollback failed and original backup was deleted")
		})
	}
}

func TestCreateFilesDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing")
	fresh := filepath.Join(dir, "fresh")
	require.NoError(t, os.WriteFile(existing, []byte("original"), 0o600))
	require.Error(t, CreateFiles(map[string][]byte{existing: []byte("replace"), fresh: []byte("fresh")}), "existing file accepted")
	got, err := os.ReadFile(existing)
	require.NoError(t, err, "original changed")
	require.Equal(t, "original", string(got), "original changed")
	{
		_, err := os.Stat(fresh)
		require.True(t, os.IsNotExist(err), "partial output")
	}
	require.NoError(t, CreateFiles(map[string][]byte{fresh: []byte("created")}))
	got, err = os.ReadFile(fresh)
	require.NoError(t, err, "new file not written")
	require.Equal(t, "created", string(got), "new file not written")
}
