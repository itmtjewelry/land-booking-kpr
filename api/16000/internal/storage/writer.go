package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// WriteJSONFileAtomic writes a JSONFile to `dir/filename` atomically, with backup.
func WriteJSONFileAtomic(dir, filename string, jf JSONFile) error {
	if dir == "" {
		return fmt.Errorf("storage dir is empty")
	}
	full := filepath.Join(dir, filename)

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// Backup if file exists
	if _, err := os.Stat(full); err == nil {
		ts := time.Now().Format("20060102_150405")
		bak := full + ".bak." + ts
		if err := copyFile(full, bak); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}

	b, err := json.Marshal(jf)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	b = append(b, '\n')

	tmp := full + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp: %w", err)
	}

	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}

	if err := os.Rename(tmp, full); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	if err := fsyncDir(filepath.Dir(full)); err != nil {
		return fmt.Errorf("fsync dir: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
