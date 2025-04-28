package networking

import (
	"github.com/pkg/errors"
	"net"
	"syscall"
)

// IsConnectionRefused - Helper function to check for connection refused errors
func IsConnectionRefused(err error) bool {
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		var syscallErr syscall.Errno
		if errors.As(netErr.Err, &syscallErr) {
			return errors.Is(syscallErr, syscall.ECONNREFUSED)
		}
	}
	return false
}
