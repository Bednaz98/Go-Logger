package securecmp

import "crypto/subtle"

// Equal compares a and b in constant time when their lengths match.
func Equal(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
