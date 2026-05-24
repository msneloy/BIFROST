package mdns

import (
	"bufio"
	"log"
	"os/exec"
	"strings"
)

func Register(primaryName, fallbackName, ip string) []func() {
	var cmds []*exec.Cmd

	if _, err := exec.LookPath("avahi-publish"); err != nil {
		log.Println("avahi-publish not found — mDNS disabled. Install avahi-daemon/avahi-utils.")
		return nil
	}

	pName := primaryName
	if !strings.HasSuffix(pName, ".local") {
		pName += ".local"
	}

	startPublish := func(name string) *exec.Cmd {
		cmd := exec.Command("avahi-publish", "-a", "-R", name, ip)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Printf("Failed to create stderr pipe for avahi-publish %s: %v", name, err)
			return nil
		}
		if err := cmd.Start(); err != nil {
			log.Printf("Failed to start avahi-publish for %s: %v", name, err)
			return nil
		}
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Printf("[avahi-publish %s] %s", name, scanner.Text())
			}
		}()
		return cmd
	}

	if cmd1 := startPublish(pName); cmd1 != nil {
		cmds = append(cmds, cmd1)
	}

	if fallbackName != "" {
		fName := fallbackName
		if !strings.HasSuffix(fName, ".local") {
			fName += ".local"
		}
		if cmd2 := startPublish(fName); cmd2 != nil {
			cmds = append(cmds, cmd2)
		}
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
