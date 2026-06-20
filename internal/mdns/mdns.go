package mdns

import (
	"bufio"
	"log"
	"os/exec"
)

func Register(ip string) []func() {
	var cmds []*exec.Cmd

	names := []string{"bifrost.local"}

	avahiPath, _ := exec.LookPath("avahi-publish")
	if avahiPath == "" {
		avahiPath = "vendor/bin/avahi-publish"
	}
	// Fallback to absolute if vendor is missing or absolute path is needed
	if _, err := exec.LookPath(avahiPath); err != nil {
		avahiPath = "/opt/bifrost/bin/avahi-publish"
	}

	startPublish := func(name string) *exec.Cmd {
		cmd := exec.Command(avahiPath, "-a", "-R", name, ip)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return nil
		}
		if err := cmd.Start(); err != nil {
			log.Printf("Failed to start mDNS for %s: %v", name, err)
			return nil
		}
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Printf("[mdns %s] %s", name, scanner.Text())
			}
		}()
		return cmd
	}

	for _, name := range names {
		if cmd := startPublish(name); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		log.Println("Warning: mDNS registration failed or avahi-publish not found.")
	}

	return []func(){
		func() {
			for _, cmd := range cmds {
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
			}
		},
	}
}
