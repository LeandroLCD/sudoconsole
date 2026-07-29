//go:build linux

package cache

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
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

// usernameBase64 is the encoded form used in the timestamp path. Exported
// for tests.
func usernameBase64(username string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(username))
}

// parseTimestampName extracts the username from a base64-encoded
// timestamp filename. Used by tests and diagnostics.
func parseTimestampName(encoded string) (string, error) {
	dec, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

// ageInSeconds is a helper used by callers.
func ageInSeconds(mtimeUnix, nowUnix int64) float64 {
	return float64(nowUnix - mtimeUnix)
}

// itoa is a tiny helper to format integers without importing strconv.
func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return strings.TrimSpace(string(buf[pos:]))
}

var _ = strconv.Itoa
