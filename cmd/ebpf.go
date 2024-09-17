package cmd

import (
	"fmt"
	"github.com/unpackdev/fdb/helpers"
	"net"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/urfave/cli/v2"
)

// EbpfCommands returns a cli.Command that manages eBPF programs using cilium/ebpf
func EbpfCommands() *cli.Command {
	return &cli.Command{
		Name:  "ebpf",
		Usage: "Manage eBPF programs",
		Subcommands: []*cli.Command{
			{
				Name:  "load",
				Usage: "Load an eBPF program onto a network interface using Cilium",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "interface",
						Usage:    "Network interface to attach the eBPF program to (e.g., eth0)",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "obj",
						Usage: "Path to the compiled eBPF object file",
						Value: "c/obj/ebpf_program.o", // Default location of compiled object file
					},
					&cli.StringFlag{
						Name:  "section",
						Usage: "eBPF section to load (e.g., xdp)",
						Value: "xdp", // Default section
					},
				},
				Action: func(c *cli.Context) error {
					iface := c.String("interface")
					objPath := c.String("obj")
					section := c.String("section")

					// Load the compiled eBPF object file
					spec, err := ebpf.LoadCollectionSpec(objPath)
					if err != nil {
						return fmt.Errorf("failed to load eBPF object: %w", err)
					}

					// Load the eBPF collection
					coll, err := ebpf.NewCollection(spec)
					if err != nil {
						return fmt.Errorf("failed to load eBPF collection: %w", err)
					}
					defer coll.Close()

					// Select the program by section
					prog := coll.Programs[section]
					if prog == nil {
						return fmt.Errorf("program section '%s' not found in eBPF object", section)
					}

					// Attach the program to the specified interface using XDP
					l, err := link.AttachXDP(link.XDPOptions{
						Program:   prog,
						Interface: helpers.IfaceIndex(iface),
					})
					if err != nil {
						return fmt.Errorf("failed to attach eBPF program to interface %s: %w", iface, err)
					}
					defer l.Close()

					fmt.Printf("eBPF program successfully loaded onto interface %s\n", iface)
					return nil
				},
			},
			{
				Name:  "interfaces",
				Usage: "List all available network interfaces and their essential details",
				Action: func(c *cli.Context) error {
					ifaces, err := net.Interfaces()
					if err != nil {
						return fmt.Errorf("failed to list network interfaces: %w", err)
					}

					fmt.Println("Available network interfaces:")

					for _, iface := range ifaces {
						// Get interface flags
						status := "down"
						if iface.Flags&net.FlagUp != 0 {
							status = "up"
						}

						// Get associated IP addresses
						addrs, err := iface.Addrs()
						if err != nil {
							fmt.Printf("  Error getting addresses for %s: %v\n", iface.Name, err)
							continue
						}

						// Collect IP addresses
						var ipList []string
						for _, addr := range addrs {
							ipList = append(ipList, addr.String())
						}

						// Display interface details with indentation
						fmt.Printf("Name: %s\n", iface.Name)
						fmt.Printf("   Index: %d\n", iface.Index)
						fmt.Printf("   MTU: %d\n", iface.MTU)
						fmt.Printf("   HardwareAddr: %s\n", iface.HardwareAddr)
						fmt.Printf("   Status: %s\n", status)
						fmt.Printf("   IPs: %s\n\n", strings.Join(ipList, ", "))
					}
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Verify if an eBPF program is loaded on a specific interface",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "interface",
						Usage:    "Network interface to check (e.g., eth0)",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					iface := c.String("interface")

					// Check if the eBPF program is attached to the interface
					xdpAttached, info, err := helpers.IsXDPAttached(iface)
					if err != nil {
						return fmt.Errorf("failed to check XDP status on interface %s: %w", iface, err)
					}

					if xdpAttached {
						fmt.Printf("An eBPF program is loaded on interface %s\n", iface)
						fmt.Println(info)
					} else {
						fmt.Printf("No eBPF program is loaded on interface %s\n", iface)
					}
					return nil
				},
			},
		},
	}
}
