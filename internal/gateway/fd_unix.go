//go:build !windows

package gateway

import "os"

func openFDCount() int {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return 0
	}
	return len(entries)
}
