package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// WriteFiles stages every output before replacing any target. Each replacement
// uses a same-directory atomic rename. Failures roll back already replaced
// files. It rejects duplicate destinations, symlinks, and non-regular targets.
// A process or machine crash between renames can leave a mixed output set.
func WriteFiles(outputs map[string][]byte) error {
	return writeFiles(outputs, os.Rename)
}

// CreateFiles writes new files without replacing any existing destination,
// including one created concurrently. Completed files are rolled back on error.
func CreateFiles(outputs map[string][]byte) error {
	for p := range outputs {
		if _, err := os.Lstat(p); err == nil {
			return fmt.Errorf("destination already exists: %s", p)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return writeFiles(outputs, func(from, to string) error { return os.Link(from, to) })
}

func writeFiles(outputs map[string][]byte, rename func(string, string) error) error {
	type staged struct {
		path, temp, backup string
		existed            bool
	}
	paths := make([]string, 0, len(outputs))
	for p := range outputs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var files []staged
	seen := map[string]bool{}
	defer func() {
		for _, f := range files {
			if f.temp != "" {
				_ = os.Remove(f.temp)
			}
			if f.backup != "" {
				_ = os.Remove(f.backup)
			}
		}
	}()
	for _, p := range paths {
		abs, e := filepath.Abs(p)
		if e != nil {
			return e
		}
		dir, e := filepath.EvalSymlinks(filepath.Dir(abs))
		if e != nil {
			return e
		}
		abs = filepath.Join(dir, filepath.Base(abs))
		if seen[abs] {
			return fmt.Errorf("duplicate output destination")
		}
		seen[abs] = true
		f := staged{path: abs}
		mode := os.FileMode(0o644)
		if info, e := os.Lstat(abs); e == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("output must be a regular file: %s", p)
			}
			f.existed = true
			mode = info.Mode().Perm()
			old, e := os.ReadFile(abs)
			if e != nil {
				return e
			}
			f.backup, e = stage(dir, old, mode)
			if e != nil {
				return e
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		files = append(files, f)
		i := len(files) - 1
		files[i].temp, e = stage(dir, outputs[p], mode)
		if e != nil {
			return e
		}
	}
	for i, f := range files {
		if e := rename(f.temp, f.path); e != nil {
			result := e
			for j := i - 1; j >= 0; j-- {
				prev := files[j]
				var re error
				if prev.existed {
					re = rename(prev.backup, prev.path)
				} else {
					re = os.Remove(prev.path)
				}
				if re != nil {
					// Preserve the only recovery copy if restoration also fails.
					files[j].backup = ""
					result = errors.Join(result, fmt.Errorf("rollback %s failed; backup retained at %s: %w", prev.path, prev.backup, re))
				}
			}
			return result
		}
	}
	return nil
}

func stage(dir string, data []byte, mode os.FileMode) (string, error) {
	f, e := os.CreateTemp(dir, ".confgen-*")
	if e != nil {
		return "", e
	}
	name := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if e = f.Chmod(mode); e != nil {
		return "", e
	}
	if _, e = f.Write(data); e != nil {
		return "", e
	}
	if e = f.Sync(); e != nil {
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	ok = true
	return name, nil
}
