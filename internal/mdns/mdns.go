package mdns

import (
	"context"
	"log"
	"os/exec"
)

func Register(ctx context.Context, ip, hostname string) (cleanup func()) {
	cmd := exec.CommandContext(ctx, "avahi-publish", "-a", "-R", hostname+".local", ip)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		log.Printf("[!] avahi-publish not found — mDNS disabled (install avahi-utils)")
		return func() {}
	}

	log.Printf("[+] mDNS registered: %s.local -> %s", hostname, ip)

	return func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}
}
