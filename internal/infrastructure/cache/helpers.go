// Package cache — shared helpers (no build tag) used by both Linux and
// macOS timestamp readers.
package cache

// ageInSeconds returns nowUnix - mtimeUnix as a float.
func ageInSeconds(mtimeUnix, nowUnix int64) float64 {
	return float64(nowUnix - mtimeUnix)
}

// itoa formats an integer as a string without importing strconv.
// Used by timestamp readers that need to embed numbers in errors
// without taking on a stdlib import.
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
	return string(buf[pos:])
}
