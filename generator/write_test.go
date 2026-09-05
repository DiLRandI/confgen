package generator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameFailureRollback(t *testing.T) {
	for _, rollbackFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "backup retained"}[rollbackFails], func(t *testing.T) {
			dir := t.TempDir()
			a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
			for _, p := range []string{a, b} {
				if e := os.WriteFile(p, []byte("original"), 0600); e != nil {
					t.Fatal(e)
				}
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
			if !errors.Is(err, failure) {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(b)
			if string(data) != "original" {
				t.Fatal("failed destination modified")
			}
			if !rollbackFails {
				data, _ := os.ReadFile(a)
				if string(data) != "original" {
					t.Fatal("first output not restored")
				}
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
			if !found {
				t.Fatal("rollback failed and original backup was deleted")
			}
		})
	}
}
