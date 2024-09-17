package helpers

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
)

// IfaceIndex Helper to get interface index by name
func IfaceIndex(iface string) int {
	i, err := net.InterfaceByName(iface)
	if err != nil {
		panic(fmt.Sprintf("could not find interface %s: %v", iface, err))
	}
	return i.Index
}

// IsXDPAttached Helper to check if XDP program is attached to interface and return additional information
func IsXDPAttached(iface string) (bool, string, error) {
	cmd := exec.Command("ip", "link", "show", iface)
	output, err := cmd.Output()
	if err != nil {
		return false, "", err
	}

	outputStr := string(output)

	// Check if the output contains "xdp" which indicates an XDP program is attached
	if strings.Contains(outputStr, "xdp") {
		// Use a regex to extract the XDP program ID (if available)
		re := regexp.MustCompile(`prog/xdp id (\d+)`)
		match := re.FindStringSubmatch(outputStr)
		if len(match) > 1 {
			progID := match[1] // XDP program ID
			return true, fmt.Sprintf("XDP program ID: %s", progID), nil
		}
		return true, "XDP program is attached, but no ID found.", nil
	}

	return false, "No XDP program attached.", nil
}
