package networking

import (
	"fmt"
	"github.com/multiformats/go-multiaddr"
)

// parseListenAddresses parses a slice of string addresses into multiaddr.Multiaddr.
func parseListenAddresses(addrs []string) ([]multiaddr.Multiaddr, error) {
	var multiAddrs []multiaddr.Multiaddr
	for _, addr := range addrs {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid multiaddress %s: %w", addr, err)
		}
		multiAddrs = append(multiAddrs, maddr)
	}
	return multiAddrs, nil
}

// addrStrings converts a slice of Multiaddrs to a slice of strings.
func addrStrings(addrs []multiaddr.Multiaddr) []string {
	strs := make([]string, len(addrs))
	for i, addr := range addrs {
		strs[i] = addr.String()
	}
	return strs
}
