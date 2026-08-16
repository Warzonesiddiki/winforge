package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

// sessionTokenHeader is the request header that must carry the per-instance
// session token on mutating requests. See ADR-002.
const sessionTokenHeader = "X-WinForge-Token"

// newSessionToken returns 32 bytes of crypto/rand encoded as base64url (no
// padding). The token is generated once per server process; restarting the
// engine rotates it.
func newSessionToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// errInvalidToken is returned when a mutation is attempted without a valid
// session token.
var errInvalidToken = errors.New("invalid or missing session token")
