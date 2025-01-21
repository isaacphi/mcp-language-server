package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	gitignore "github.com/sabhiram/go-gitignore"
)

// WorkspaceWatcher manages file watching and version tracking
type WorkspaceWatcher struct {
	client        *lsp.Client
	ignore        *gitignore.GitIgnore
	workspacePath string

	// Debouncing related fields
	debounceTime time.Duration
	debounceMap  map[string]*time.Timer
	debounceMu   sync.Mutex
}

// NewWorkspaceWatcher creates a new instance of WorkspaceWatcher
func NewWorkspaceWatcher(client *lsp.Client) *WorkspaceWatcher {
	return &WorkspaceWatcher{
		client:       client,
		debounceTime: 300 * time.Millisecond, // Configurable debounce duration
		debounceMap:  make(map[string]*time.Timer),
	}
}

func (w *WorkspaceWatcher) loadGitIgnore(workspacePath string) error {
	gitignorePath := filepath.Join(workspacePath, ".gitignore")

	// Read and log the content of .gitignore
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		log.Printf("DEBUG: Error reading .gitignore: %v", err)
		return fmt.Errorf("error reading gitignore: %w", err)
	}
	log.Printf("DEBUG: .gitignore content:\n%s", string(content))

	ignore, err := gitignore.CompileIgnoreFile(gitignorePath)
	if err != nil {
		return fmt.Errorf("error compiling gitignore: %w", err)
	}
	w.ignore = ignore

	log.Printf("DEBUG: Successfully loaded .gitignore")
	return nil
}

func (w *WorkspaceWatcher) shouldIgnorePath(path string, workspacePath string) bool {
	// Always ignore .git directory
	if filepath.Base(path) == ".git" {
		log.Printf("DEBUG: Ignoring .git directory: %s", path)
		return true
	}

	// If we have a gitignore file, check against its patterns
	if w.ignore != nil {
		// Convert to relative path for gitignore matching
		relPath, err := filepath.Rel(workspacePath, path)
		if err != nil {
			log.Printf("DEBUG: Error getting relative path for %s: %v", path, err)
			return false
		}

		// Convert path separators to forward slashes
		relPath = filepath.ToSlash(relPath)

		// Remove leading ./ if present
		relPath = strings.TrimPrefix(relPath, "./")

		matches, pattern := w.ignore.MatchesPathHow(relPath)

		log.Printf("DEBUG: Path check details:")
		log.Printf("  Original path: %s", path)
		log.Printf("  Workspace: %s", workspacePath)
		log.Printf("  Relative path: %s", relPath)
		log.Printf("  Matches? %v", matches)
		if pattern != nil {
			log.Printf("  Matched pattern: %s (line %d)", pattern.Line, pattern.LineNo)
		}

		return matches
	}

	return false
}

func (w *WorkspaceWatcher) watchWorkspace(ctx context.Context, workspacePath string) {
	w.workspacePath = workspacePath

	// Load gitignore patterns
	if err := w.loadGitIgnore(workspacePath); err != nil {
		log.Printf("Error loading gitignore: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error creating watcher: %v", err)
	}
	defer watcher.Close()

	// Watch all subdirectories except ignored ones
	err = filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Check if directory should be ignored
			if w.shouldIgnorePath(path, workspacePath) {
				return filepath.SkipDir
			}

			err = watcher.Add(path)
			if err != nil {
				log.Printf("Error watching path %s: %v", path, err)
			}
		} else {
			if w.shouldIgnorePath(path, workspacePath) {
				return nil
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking workspace: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			log.Println("EVENT::", event, ok)

			if !ok {
				return
			}

			// Skip temporary files and backup files
			if strings.HasSuffix(event.Name, "~") || strings.HasSuffix(event.Name, ".swp") {
				log.Println("Skipping ~")
				continue
			}

			// Skip ignored paths
			if w.shouldIgnorePath(event.Name, workspacePath) {
				log.Println("Skipping", event.Name)
				continue
			}

			uri := fmt.Sprintf("file://%s", event.Name)

			switch {
			case event.Op&fsnotify.Write != 0:
				w.debounceHandleChange(ctx, uri)
			case event.Op&fsnotify.Create != 0:
				// Also watch new directories
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if !w.shouldIgnorePath(event.Name, workspacePath) {
						err = watcher.Add(event.Name)
						if err != nil {
							log.Printf("Error watching new directory: %v", err)
						}
					}
				}
				w.debounceHandleCreate(ctx, uri)
			case event.Op&fsnotify.Remove != 0:
				w.handleDelete(ctx, uri)
			case event.Op&fsnotify.Rename != 0:
				w.handleDelete(ctx, uri)
				if info, err := os.Stat(event.Name); err == nil {
					if info.IsDir() && !w.shouldIgnorePath(event.Name, workspacePath) {
						err = watcher.Add(event.Name)
						if err != nil {
							log.Printf("Error watching new directory: %v", err)
						}
					}
					w.debounceHandleCreate(ctx, uri)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v\n", err)
		}
	}
}

func (w *WorkspaceWatcher) debounceHandleChange(ctx context.Context, uri string) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Cancel existing timer if any
	if timer, exists := w.debounceMap[uri]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceMap[uri] = time.AfterFunc(w.debounceTime, func() {
		w.handleChange(ctx, uri)

		// Cleanup timer after execution
		w.debounceMu.Lock()
		delete(w.debounceMap, uri)
		w.debounceMu.Unlock()
	})
}

func (w *WorkspaceWatcher) debounceHandleCreate(ctx context.Context, uri string) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Cancel existing timer if any
	if timer, exists := w.debounceMap[uri]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceMap[uri] = time.AfterFunc(w.debounceTime, func() {
		w.handleCreate(ctx, uri)

		// Cleanup timer after execution
		w.debounceMu.Lock()
		delete(w.debounceMap, uri)
		w.debounceMu.Unlock()
	})
}

func (w *WorkspaceWatcher) handleCreate(ctx context.Context, uri string) {
	_, err := os.ReadFile(uri[7:]) // Remove "file://" prefix
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return
	}

	// Temporarily open the file to trigger analysis
	if err := w.client.OpenFile(ctx, uri[7:]); err != nil {
		log.Printf("Error opening file for analysis: %v", err)
		return
	}

	// Close it right after to not keep it in memory
	log.Printf("Error closing file after analysis: %v", err)
	if err := w.client.CloseFile(ctx, uri[7:]); err != nil {
		log.Printf("Error closing file: %v", err)
	}
}

func (w *WorkspaceWatcher) handleChange(ctx context.Context, uri string) {
	// Only handle changes for files that aren't "open"
	// If a file is open, changes should come through the client's NotifyChange
	if w.client.IsFileOpen(uri[7:]) { // Remove "file://" prefix
		err := w.client.NotifyChange(ctx, uri[7:])
		if err != nil {
			log.Printf("Error notifying change: %v", err)
		}
		return
	}

	// Skip temporary files and backup files
	if strings.HasSuffix(uri, "~") || strings.HasSuffix(uri, ".swp") {
		return
	}

	// Temporarily open file to trigger analysis
	if err := w.client.OpenFile(ctx, uri[7:]); err != nil {
		log.Printf("Error opening file for analysis: %v", err)
		return
	}

	// And immediately close it
	if err := w.client.CloseFile(ctx, uri[7:]); err != nil {
		log.Printf("Error closing file after analysis: %v", err)
	}
}

func (w *WorkspaceWatcher) handleDelete(ctx context.Context, uri string) {
	// If the file is open in the client, close it properly
	if w.client.IsFileOpen(uri[7:]) {
		if err := w.client.CloseFile(ctx, uri[7:]); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}
}
