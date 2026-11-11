package uploads

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/trae/runninghub2api/internal/store"
)

// Cleaner removes local upload files after tasks finish (and orphan files).
type Cleaner struct {
	store         *store.Store
	dataDir       string
	defaultHours  int
	interval      time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
	mu            sync.Mutex
	running       bool
}

func NewCleaner(st *store.Store, dataDir string, defaultHours int, interval time.Duration) *Cleaner {
	if defaultHours < 0 {
		defaultHours = 0
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Cleaner{
		store:        st,
		dataDir:      dataDir,
		defaultHours: store.NormalizeUploadKeepHours(defaultHours),
		interval:     interval,
		stopCh:       make(chan struct{}),
	}
}

func (c *Cleaner) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return
	}
	c.running = true
	c.wg.Add(1)
	go c.loop()
}

func (c *Cleaner) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	close(c.stopCh)
	c.mu.Unlock()
	c.wg.Wait()
}

func (c *Cleaner) keepFor() time.Duration {
	h := c.defaultHours
	if c.store != nil {
		h = c.store.UploadKeepHours(c.defaultHours)
	}
	return time.Duration(h) * time.Hour
}

func (c *Cleaner) loop() {
	defer c.wg.Done()
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-timer.C:
			if n, err := c.RunOnce(); err != nil {
				log.Printf("upload cleaner: %v", err)
			} else if n > 0 {
				log.Printf("upload cleaner: removed %d file(s) (keep=%dh)", n, int(c.keepFor().Hours()))
			}
			timer.Reset(c.interval)
		}
	}
}

func (c *Cleaner) uploadsDir() string {
	return filepath.Join(c.dataDir, "uploads")
}

func (c *Cleaner) underUploads(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	up := filepath.Clean(c.uploadsDir())
	rel, err := filepath.Rel(up, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// RunOnce deletes expired terminal-task uploads and orphan files. Returns deleted count.
func (c *Cleaner) RunOnce() (int, error) {
	upDir := c.uploadsDir()
	if err := os.MkdirAll(upDir, 0o755); err != nil {
		return 0, err
	}
	keep := c.keepFor()
	cutoff := time.Now().UTC().Add(-keep).Format(time.RFC3339)
	active, err := c.store.ListActiveUploadPaths()
	if err != nil {
		return 0, err
	}
	deleted := 0

	tasks, err := c.store.ListTerminalUploadsBefore(cutoff)
	if err != nil {
		return 0, err
	}
	for _, t := range tasks {
		path := strings.TrimSpace(t.ImagePath)
		if path == "" {
			continue
		}
		if _, keepActive := active[path]; keepActive {
			continue
		}
		if c.underUploads(path) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Printf("upload cleaner: remove %s: %v", path, err)
			} else {
				deleted++
			}
		}
		if err := c.store.ClearTaskImagePath(t.ID); err != nil {
			log.Printf("upload cleaner: clear task %s: %v", t.ID, err)
		}
	}

	entries, err := os.ReadDir(upDir)
	if err != nil {
		return deleted, err
	}
	activeBase := make(map[string]struct{}, len(active))
	for p := range active {
		activeBase[filepath.Base(p)] = struct{}{}
	}
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, keepActive := activeBase[name]; keepActive {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) < keep {
			continue
		}
		full := filepath.Join(upDir, name)
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			log.Printf("upload cleaner: orphan %s: %v", full, err)
			continue
		}
		deleted++
	}
	return deleted, nil
}
