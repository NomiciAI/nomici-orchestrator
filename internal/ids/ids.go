package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a compact random identifier with the given prefix.
func New(prefix string) string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(fmt.Errorf("generate id: %w", err))
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}
