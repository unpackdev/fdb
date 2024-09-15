package cmd

import (
	"fmt"
	"github.com/unpackdev/fdb/certs"
	"github.com/urfave/cli/v2"
	"os"
)

// CertsCommand returns a cli.Command that generates a self-signed certificate
func CertsCommand() *cli.Command {
	return &cli.Command{
		Name:  "certs",
		Usage: "Manage SSL certificates",
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create a new certificate",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "domain",
						Usage:    "Domain name for the certificate",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "email",
						Usage:    "Email for Let's Encrypt notifications",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					domain := c.String("domain")
					email := c.String("email")
					return certs.CertbotCreateCertificate(domain, email)
				},
			},
			{
				Name:  "renew",
				Usage: "Renew existing certificates",
				Action: func(c *cli.Context) error {
					return certs.CertbotRenewCertificate()
				},
			},
			{
				Name:  "local",
				Usage: "Generate self-signed certificates for testing purposes",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "cert",
						Usage: "Path to save the certificate PEM file",
						Value: "cert.pem",
					},
					&cli.StringFlag{
						Name:  "key",
						Usage: "Path to save the private key PEM file",
						Value: "key.pem",
					},
				},
				Action: func(c *cli.Context) error {
					cert, privKey, err := certs.GenerateSelfSignedCert() // Use the correct function
					if err != nil {
						return fmt.Errorf("failed to generate certificate: %w", err)
					}

					// Export the cert and key to PEM format
					certPEM, keyPEM, err := certs.ExportPEM(cert, privKey)
					if err != nil {
						return fmt.Errorf("failed to export certificate and key: %w", err)
					}

					// Save the PEM files
					certOutput := c.String("cert")
					keyOutput := c.String("key")

					if err := os.WriteFile(certOutput, certPEM, 0644); err != nil {
						return fmt.Errorf("failed to write certificate file: %w", err)
					}
					if err := os.WriteFile(keyOutput, keyPEM, 0600); err != nil {
						return fmt.Errorf("failed to write private key file: %w", err)
					}

					fmt.Printf("Certificate and key have been successfully generated and saved to %s and %s\n", certOutput, keyOutput)
					return nil
				},
			},
		},
	}
}
