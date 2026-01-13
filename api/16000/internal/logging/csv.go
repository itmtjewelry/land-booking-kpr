package logging

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const csvHeader = "timestamp,level,service,action,user_id,entity_type,entity_id,message\n"

type CSVLogger struct {
	mu        sync.Mutex
	baseDir   string
	service   string
	localTZ   *time.Location
}

func NewCSVLogger(baseDir, service string) *CSVLogger {
	// Use local timezone for DDMMYYYY naming.
	loc := time.Local
	return &CSVLogger{
		baseDir: baseDir,
		service: service,
		localTZ: loc,
	}
}

func (l *CSVLogger) filePathForNow() string {
	now := time.Now().In(l.localTZ)
	ddmmyyyy := now.Format("02012006") // DDMMYYYY
	return filepath.Join(l.baseDir, fmt.Sprintf("app-%s.csv", ddmmyyyy))
}

func (l *CSVLogger) ensureHeader(path string) error {
	// If file exists and has size > 0, assume header already present.
	fi, err := os.Stat(path)
	if err == nil && fi.Size() > 0 {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// If file newly created or empty, write header.
	// Re-check size after open.
	fi, err = f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		if _, err := f.WriteString(csvHeader); err != nil {
			return err
		}
	}
	return nil
}

func (l *CSVLogger) Log(level, action, userID, entityType, entityID, message string) {
	// Non-blocking by design: failures should not crash core logic.
	defer func() { _ = recover() }()

	l.mu.Lock()
	defer l.mu.Unlock()

	path := l.filePathForNow()
	if err := l.ensureHeader(path); err != nil {
		// Best-effort: cannot log, so just return.
		return
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	ts := time.Now().In(l.localTZ).Format(time.RFC3339)

	// CSV escaping minimal: wrap message in quotes and escape internal quotes.
	escapedMsg := `"` + escapeQuotes(message) + `"`
	line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s\n",
		ts, level, l.service, action, userID, entityType, entityID, escapedMsg,
	)

	_, _ = w.WriteString(line)
	_ = w.Flush()
}

func escapeQuotes(s string) string {
	// escape " -> ""
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '"' {
			out = append(out, '"', '"')
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}
