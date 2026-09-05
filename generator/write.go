package generator

import (
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
		mode := os.FileMode(0644)
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
		if e := os.Rename(f.temp, f.path); e != nil {
			for j := i - 1; j >= 0; j-- {
				prev := files[j]
				var re error
				if prev.existed {
					re = os.Rename(prev.backup, prev.path)
				} else {
					re = os.Remove(prev.path)
				}
				if re != nil {
					return fmt.Errorf("replace output: %w; rollback %s: %v", e, prev.path, re)
				}
			}
			return e
		}
	}
	return nil
}

func stage(dir string, data []byte, mode os.FileMode) (string, error) {
	f, e := os.CreateTemp(dir, ".configgen-*")
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
