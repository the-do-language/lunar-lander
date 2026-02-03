package watch

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type ScriptWatcher struct {
	path     string
	debounce time.Duration
	onChange func()
}

func NewScriptWatcher(path string, onChange func()) *ScriptWatcher {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return &ScriptWatcher{
		path:     filepath.Clean(absPath),
		debounce: 200 * time.Millisecond,
		onChange: onChange,
	}
}

func (s *ScriptWatcher) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	dir := filepath.Dir(s.path)
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watch directory: %w", err)
	}

	var (
		timer    *time.Timer
		debounce <-chan time.Time
	)
	trigger := func() {
		if s.onChange != nil {
			s.onChange()
		}
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Name != s.path {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if timer != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			timer = time.NewTimer(s.debounce)
			debounce = timer.C
		case err := <-watcher.Errors:
			if err != nil {
				return fmt.Errorf("watch error: %w", err)
			}
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil
		case <-debounce:
			timer = nil
			debounce = nil
			trigger()
		}
	}
}
