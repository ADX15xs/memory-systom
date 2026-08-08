// Package watch provides hot-reload file watching using fsnotify.
package watch

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Reloader is called when a relevant file change is detected.
type Reloader func() error

// Watcher monitors the knowledge base directory for file changes
// and triggers a reload callback.
type Watcher struct {
	watcher *fsnotify.Watcher
	rootDir string
	reload  Reloader
	done    chan struct{}
}

// New creates and starts a file watcher.
// It watches the root directory recursively for .yaml file changes.
func New(rootDir string, reload Reloader) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	fw := &Watcher{
		watcher: w,
		rootDir: rootDir,
		reload:  reload,
		done:    make(chan struct{}),
	}

	// Add all subdirectories recursively
	if err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories and .git
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			return w.Add(path)
		}
		return nil
	}); err != nil {
		w.Close()
		return nil, fmt.Errorf("walk directories for watch: %w", err)
	}

	go fw.loop()
	return fw, nil
}

func (fw *Watcher) loop() {
	defer close(fw.done)

	// Debounce: collect events within 100ms and batch reload
	const debounceMs = 100
	var timer *time.Timer
	debounceCh := make(chan struct{}, 1)

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			// Only react to .yaml files
			if !strings.HasSuffix(strings.ToLower(event.Name), ".yaml") {
				continue
			}

			// Debounce: reset timer on each event
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceMs*time.Millisecond, func() {
				select {
				case debounceCh <- struct{}{}:
				default:
				}
			})

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("file watcher error: %v", err)

		case <-debounceCh:
			log.Printf("file change detected, reloading...")
			if err := fw.reload(); err != nil {
				log.Printf("reload after file change: %v", err)
			}

		case <-fw.done:
			return
		}
	}
}

// Stop stops the file watcher.
func (fw *Watcher) Stop() {
	fw.watcher.Close()
	<-fw.done
}
