package watcher

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// Start monitors the specified directories for modifications in .go, .html, and .tmpl files.
// When a change is detected, it rebuilds the binary using buildCmd and restarts the process.
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

				// syscall.Exec replaces the current process with the newly built binary
				err = syscall.Exec(execPath, os.Args, os.Environ())
				if err != nil {
					log.Printf("[watcher] syscall.Exec failed: %v", err)
				}
			}
		}
	}()
}
