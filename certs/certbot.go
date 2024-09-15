package certs

import (
	"fmt"
	"os/exec"
)

// CertbotCreateCertificate runs certbot to create a certificate
func CertbotCreateCertificate(domain, email string) error {
	cmd := exec.Command("certbot", "certonly", "--non-interactive", "--agree-tos", "--email", email, "-d", domain)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create certificate: %s", string(output))
	}
	fmt.Println("Certificate created successfully:", string(output))
	return nil
}

// CertbotRenewCertificate runs certbot to renew a certificate
func CertbotRenewCertificate() error {
	cmd := exec.Command("certbot", "renew")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to renew certificate: %s", string(output))
	}
	fmt.Println("Certificate renewed successfully:", string(output))
	return nil
}
