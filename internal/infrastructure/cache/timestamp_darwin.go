//go:build darwin

package cache

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"syscall"
)

// timestampPath returns the path to the sudo timestamp file on macOS.
//
// Format: /var/run/sudo/ts/<base64-encoded-username>
// macOS uses standard base64 (with padding stripped) — historically the
// encoding matches `btoa` minus padding characters.
func timestampPath(username string) string {
	return "/var/run/sudo/ts/" + encodeUsername(username)
}

// encodeUsername matches macOS sudo's quirky base64 variant: standard
// alphabet, padding stripped.
func encodeUsername(username string) string {
	return strings.TrimRight(base64.StdEncoding.EncodeToString([]byte(username)), "=")
}

// readTimestamp returns the mtime of the timestamp file, or 0 if it
// does not exist.
func readTimestamp(username string) (int64, error) {
	path := timestampPath(username)
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat timestamp: %w", err)
	}
	return fi.ModTime().Unix(), nil
}

// writeTimestampRefreshed touches the file. Best-effort.
func writeTimestampRefreshed(username string) error {
	path := timestampPath(username)
	now := timeNow()
	return os.Chtimes(path, now, now)
}

// currentUsername returns the effective user.
func currentUsername() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return ""
}

var _ = syscall.Getuid
