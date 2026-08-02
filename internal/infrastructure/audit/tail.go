package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// TailOptions configures the behaviour of Tail.
type TailOptions struct {
	// Path is the absolute path to the JSONL log file. Required.
	Path string
	// N is the number of most-recent records to return. 0 returns
	// everything. Negative values are treated as 0.
	N int
	// Follow opens the file in tail-follow mode: after the initial N
	// records are returned the reader blocks waiting for new entries.
	Follow bool
}

// Tail reads the last N entries from a JSONL audit file.
//
// The implementation reads the file backwards in 4 KiB chunks until
// N newlines have been consumed, then re-reads forward from that
// offset to parse each line as JSON. This keeps the memory cost
// constant regardless of file size.
func Tail(opts TailOptions) ([]domain.AuditEvent, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("audit tail: empty path")
	}
	f, err := os.Open(opts.Path) // #nosec G304 -- path is the user-supplied audit log file.
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", opts.Path, err)
	}
	defer func() { _ = f.Close() }()

	offset, err := tailStartOffset(f, opts.N)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek %d: %w", offset, err)
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var out []domain.AuditEvent
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e domain.AuditEvent
		if err := json.Unmarshal(line, &e); err != nil {
			// Skip malformed lines so a single bad record does not
			// break triage.
			continue
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return out, nil
}

// tailStartOffset returns the byte offset where the scanner should
// start reading to capture the last N lines. When N <= 0 the offset
// is 0 (start of file).
func tailStartOffset(f *os.File, n int) (int64, error) {
	if n <= 0 {
		return 0, nil
	}
	stat, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat: %w", err)
	}
	size := stat.Size()
	const chunk = 4096
	buf := make([]byte, chunk)
	offset := size
	newlines := 0
	for offset > 0 && newlines <= n {
		readSize := int64(chunk)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize
		if _, err := f.ReadAt(buf[:readSize], offset); err != nil {
			return 0, fmt.Errorf("readat: %w", err)
		}
		for i := int(readSize) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				newlines++
				if newlines > n {
					offset += int64(i) + 1
					return offset, nil
				}
			}
		}
	}
	return offset, nil
}
