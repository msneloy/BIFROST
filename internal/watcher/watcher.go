package watcher

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// Start monitors the specified directories for modifications in .go, .html,
// and .tmpl files. When a change is detected, it rebuilds the binary using
// buildCmd and restarts the process via syscall.Exec (Unix) or a new process
// spawn (Windows).
func Start(dirs []string, buildCmd []string) {
	files := make(map[string]time.Time)
	initialized := false

	// scan checks for modified, added, or deleted files.
	scan := func() bool {
		changed := false
		currentFiles := make(map[string]bool)

		for _, dir := range dirs {
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				ext := filepath.Ext(path)
				if ext == ".go" || ext == ".html" || ext == ".tmpl" {
					currentFiles[path] = true
					lastMod, exists := files[path]
					if !exists {
						files[path] = info.ModTime()
						if initialized {
							changed = true
						}
					} else if info.ModTime().After(lastMod) {
						files[path] = info.ModTime()
						changed = true
					}
				}
				return nil
			})
		}

		// Detect deleted files
		for path := range files {
			if !currentFiles[path] {
				delete(files, path)
				changed = true
			}
		}

		return changed
	}

	// Populate initial file map
	_ = scan()
	initialized = true

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if scan() {
				log.Println("[watcher] Code change detected. Rebuilding BIFROST...")
				
				cmd := exec.Command(buildCmd[0], buildCmd[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				
				if err := cmd.Run(); err != nil {
					log.Printf("[watcher] Build failed: %v", err)
					continue
				}

				log.Println("[watcher] Build successful! Restarting process...")
				
				execPath, err := os.Executable()
				if err != nil {
					log.Printf("[watcher] Failed to find executable path: %v", err)
					continue
				}

				if runtime.GOOS != "windows" {
					// On Unix, use syscall.Exec to replace the current process.
					// This preserves the PID and terminal job control (Ctrl+C works).
					log.Printf("[watcher] Performing syscall.Exec: %s", execPath)
					err = syscall.Exec(execPath, os.Args, os.Environ())
					if err != nil {
						log.Printf("[watcher] syscall.Exec failed: %v", err)
						continue
					}
				} else {
					// Windows fallback (cannot use syscall.Exec)
					newCmd := exec.Command(execPath, os.Args[1:]...)
					newCmd.Stdout = os.Stdout
					newCmd.Stderr = os.Stderr
					newCmd.Stdin = os.Stdin
					newCmd.Env = os.Environ()

					if err := newCmd.Start(); err != nil {
						log.Printf("[watcher] Failed to restart: %v", err)
						continue
					}
					os.Exit(0)
				}
			}
		}
	}()
}
