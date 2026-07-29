//go:build linux

package cache

import (
	"encoding/base64"
	"fmt"
	"os"
)

// timestampPath returns the path to the sudo timestamp file for the
// given user on Linux.
//
// Format: /var/db/sudo/ts/<base64-url-encoded-username>
// The encoding uses URL-safe base64 without padding.
func timestampPath(username string) string {
	return "/var/db/sudo/ts/" + base64.RawURLEncoding.EncodeToString([]byte(username))
}

// readTimestamp returns the mtime of the timestamp file, or 0 if it
// does not exist. The caller decides whether mtime is recent enough.
func readTimestamp(username string) (mtimeUnix int64, err error) {
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

// writeTimestampRefreshed refreshes the timestamp by touching the file.
// This is a best-effort fallback when sudo -v is not available.
func writeTimestampRefreshed(username string) error {
	path := timestampPath(username)
	now := timeNow()
	return os.Chtimes(path, now, now)
}

// currentUsername returns the effective user running the process.
func currentUsername() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return ""
}
