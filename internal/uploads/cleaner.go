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
			files, tasks, jobs, err := c.RunOnce()
			if err != nil {
				log.Printf("retention cleaner: %v", err)
			} else if files > 0 || tasks > 0 || jobs > 0 {
				log.Printf("retention cleaner: files=%d tasks=%d register_jobs=%d (keep=%dh)",
					files, tasks, jobs, int(c.keepFor().Hours()))
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

// RunOnce deletes expired uploads, terminal task rows, and finished register jobs.
// Returns deleted file / task / register-job counts.
func (c *Cleaner) RunOnce() (files, tasks, jobs int, err error) {
	upDir := c.uploadsDir()
	if err = os.MkdirAll(upDir, 0o755); err != nil {
		return 0, 0, 0, err
	}
	keep := c.keepFor()
	cutoff := time.Now().UTC().Add(-keep).Format(time.RFC3339)
	active, err := c.store.ListActiveUploadPaths()
	if err != nil {
		return 0, 0, 0, err
	}

	uploadTasks, err := c.store.ListTerminalUploadsBefore(cutoff)
	if err != nil {
		return 0, 0, 0, err
	}
	for _, t := range uploadTasks {
		path := strings.TrimSpace(t.ImagePath)
		if path == "" {
			continue
		}
		if _, keepActive := active[path]; keepActive {
			continue
		}
		if c.underUploads(path) {
			if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
				log.Printf("retention cleaner: remove %s: %v", path, rmErr)
			} else {
				files++
			}
		}
		if clearErr := c.store.ClearTaskImagePath(t.ID); clearErr != nil {
			log.Printf("retention cleaner: clear task %s: %v", t.ID, clearErr)
		}
	}

	entries, err := os.ReadDir(upDir)
	if err != nil {
		return files, 0, 0, err
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
		if rmErr := os.Remove(full); rmErr != nil && !os.IsNotExist(rmErr) {
			log.Printf("retention cleaner: orphan %s: %v", full, rmErr)
			continue
		}
		files++
	}

	// keep=0 means "clean upload files ASAP" only — never wipe task/register history.
	// Record purge requires an explicit positive retention window.
	if keep <= 0 {
		return files, 0, 0, nil
	}

	terminal, err := c.store.ListTerminalTasksBefore(cutoff, 500)
	if err != nil {
		return files, 0, 0, err
	}
	if len(terminal) > 0 {
		ids := make([]string, 0, len(terminal))
		for _, t := range terminal {
			path := strings.TrimSpace(t.ImagePath)
			if path != "" {
				if _, keepActive := active[path]; !keepActive && c.underUploads(path) {
					if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
						log.Printf("retention cleaner: task-file %s: %v", path, rmErr)
					} else {
						files++
					}
				}
			}
			ids = append(ids, t.ID)
		}
		tasks, err = c.store.DeleteTasksByIDs(ids)
		if err != nil {
			return files, tasks, 0, err
		}
	}

	jobs, err = c.store.PurgeFinishedRegisterJobsBefore(cutoff, 200)
	if err != nil {
		return files, tasks, jobs, err
	}
	return files, tasks, jobs, nil
}
