package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// Create a logger for the watcher component
var watcherLogger = logging.NewLogger(logging.Watcher)

// WorkspaceWatcher manages LSP file watching
type WorkspaceWatcher struct {
	client        LSPClient
	workspacePath string

	config      *WatcherConfig
	debounceMap map[string]*time.Timer
	debounceMu  sync.Mutex

	// File watchers registered by the server
	registrations  []protocol.FileSystemWatcher
	registrationMu sync.RWMutex

	// Gitignore matcher
	gitignore *GitignoreMatcher
}

// NewWorkspaceWatcher creates a new workspace watcher with default configuration
func NewWorkspaceWatcher(client LSPClient) *WorkspaceWatcher {
	return NewWorkspaceWatcherWithConfig(client, DefaultWatcherConfig())
}

// NewWorkspaceWatcherWithConfig creates a new workspace watcher with custom configuration
func NewWorkspaceWatcherWithConfig(client LSPClient, config *WatcherConfig) *WorkspaceWatcher {
	return &WorkspaceWatcher{
		client:        client,
		config:        config,
		debounceMap:   make(map[string]*time.Timer),
		registrations: []protocol.FileSystemWatcher{},
	}
}

// AddRegistrations adds file watchers to track
func (w *WorkspaceWatcher) AddRegistrations(ctx context.Context, id string, watchers []protocol.FileSystemWatcher) {
	w.registrationMu.Lock()
	defer w.registrationMu.Unlock()

	// Add new watchers
	w.registrations = append(w.registrations, watchers...)

	// Log registration information
	watcherLogger.Info("Added %d file watcher registrations (id: %s), total: %d",
		len(watchers), id, len(w.registrations))

	// Detailed debug information about registrations
	if watcherLogger.IsLevelEnabled(logging.LevelDebug) {
		for i, watcher := range watchers {
			watcherLogger.Debug("Registration #%d raw data:", i+1)

			// Log the GlobPattern
			switch v := watcher.GlobPattern.Value.(type) {
			case string:
				watcherLogger.Debug("  GlobPattern: string pattern '%s'", v)
			case protocol.RelativePattern:
				watcherLogger.Debug("  GlobPattern: RelativePattern with pattern '%s'", v.Pattern)

				// Log BaseURI details
				switch u := v.BaseURI.Value.(type) {
				case string:
					watcherLogger.Debug("    BaseURI: string '%s'", u)
				case protocol.DocumentUri:
					watcherLogger.Debug("    BaseURI: DocumentUri '%s'", u)
				default:
					watcherLogger.Debug("    BaseURI: unknown type %T", u)
				}
			default:
				watcherLogger.Debug("  GlobPattern: unknown type %T", v)
			}

			// Log WatchKind
			watchKind := protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
			if watcher.Kind != nil {
				watchKind = *watcher.Kind
			}
			watcherLogger.Debug("  WatchKind: %d (Create:%v, Change:%v, Delete:%v)",
				watchKind,
				watchKind&protocol.WatchCreate != 0,
				watchKind&protocol.WatchChange != 0,
				watchKind&protocol.WatchDelete != 0)

			// Test match against some example paths
			testPaths := []string{
				"/Users/phil/dev/mcp-language-server/internal/watcher/watcher.go",
				"/Users/phil/dev/mcp-language-server/go.mod",
			}

			for _, testPath := range testPaths {
				isMatch := w.matchesPattern(testPath, watcher.GlobPattern)
				watcherLogger.Debug("  Test path '%s': %v", testPath, isMatch)
			}
		}
	}

	// Find and open all existing files that match the newly registered patterns
	// TODO: not all language servers require this, but typescript does. Make this configurable
	go func() {
		startTime := time.Now()
		filesOpened := 0

		err := filepath.WalkDir(w.workspacePath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip directories that should be excluded
			if d.IsDir() {
				watcherLogger.Debug("Processing directory: %s", path)
				if path != w.workspacePath && w.shouldExcludeDir(path) {
					watcherLogger.Debug("Skipping excluded directory: %s", path)
					return filepath.SkipDir
				}
			} else {
				// Process files
				w.openMatchingFile(ctx, path)
				filesOpened++

				// Add a small delay after every 100 files to prevent overwhelming the server
				if filesOpened%100 == 0 {
					time.Sleep(10 * time.Millisecond)
				}
			}

			return nil
		})

		elapsedTime := time.Since(startTime)
		watcherLogger.Info("Workspace scan complete: processed %d files in %.2f seconds",
			filesOpened, elapsedTime.Seconds())

		if err != nil {
			watcherLogger.Error("Error scanning workspace for files to open: %v", err)
		}
	}()
}

// WatchWorkspace sets up file watching for a workspace
func (w *WorkspaceWatcher) WatchWorkspace(ctx context.Context, workspacePath string) {
	w.workspacePath = workspacePath

	// Initialize gitignore matcher
	gitignore, err := NewGitignoreMatcher(workspacePath)
	if err != nil {
		watcherLogger.Error("Error initializing gitignore matcher: %v", err)
	} else {
		w.gitignore = gitignore
		watcherLogger.Info("Initialized gitignore matcher for %s", workspacePath)
	}

	// Register handler for file watcher registrations from the server
	lsp.RegisterFileWatchHandler(func(id string, watchers []protocol.FileSystemWatcher) {
		w.AddRegistrations(ctx, id, watchers)
	})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		watcherLogger.Fatal("Error creating watcher: %v", err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			watcherLogger.Error("Error closing watcher: %v", err)
		}
	}()

	// Watch the workspace recursively
	err = filepath.WalkDir(workspacePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories (except workspace root)
		if d.IsDir() && path != workspacePath {
			if w.shouldExcludeDir(path) {
				watcherLogger.Debug("Skipping watching excluded directory: %s", path)
				return filepath.SkipDir
			}
		}

		// Add directories to watcher
		if d.IsDir() {
			err = watcher.Add(path)
			if err != nil {
				watcherLogger.Error("Error watching path %s: %v", path, err)
			}
		}

		return nil
	})

	if err != nil {
		watcherLogger.Fatal("Error walking workspace: %v", err)
	}

	// Event loop
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			uri := fmt.Sprintf("file://%s", event.Name)

			// Check if this is a file (not a directory) and should be excluded
			isFile := false
			isExcluded := false

			if info, err := os.Stat(event.Name); err == nil {
				isFile = !info.IsDir()
				if isFile {
					isExcluded = w.shouldExcludeFile(event.Name)
					if isExcluded {
						watcherLogger.Debug("Skipping excluded file: %s", event.Name)
					}
				} else {
					// It's a directory
					isExcluded = w.shouldExcludeDir(event.Name)
					if isExcluded {
						watcherLogger.Debug("Skipping excluded directory: %s", event.Name)
					}
				}
			}

			// Add new directories to the watcher
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil {
					if info.IsDir() {
						// Skip excluded directories
						if !w.shouldExcludeDir(event.Name) {
							if err := watcher.Add(event.Name); err != nil {
								watcherLogger.Error("Error watching new directory: %v", err)
							}
						}
					} else {
						// For newly created files
						if !w.shouldExcludeFile(event.Name) {
							w.openMatchingFile(ctx, event.Name)
						}
					}
				}
			}

			// Debug logging
			if watcherLogger.IsLevelEnabled(logging.LevelDebug) {
				matched, kind := w.isPathWatched(event.Name)
				watcherLogger.Debug("Event: %s, Op: %s, Watched: %v, Kind: %d, Excluded: %v",
					event.Name, event.Op.String(), matched, kind, isExcluded)
			}

			// Skip excluded files from further processing
			if isExcluded {
				continue
			}

			// Check if this path should be watched according to server registrations
			if watched, watchKind := w.isPathWatched(event.Name); watched {
				switch {
				case event.Op&fsnotify.Write != 0:
					if watchKind&protocol.WatchChange != 0 {
						w.debounceHandleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Changed))
					}
				case event.Op&fsnotify.Create != 0:
					// Already handled earlier in the event loop
					// Just send the notification if needed
					info, _ := os.Stat(event.Name)
					if info != nil && !info.IsDir() && watchKind&protocol.WatchCreate != 0 {
						w.debounceHandleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Created))
					}
				case event.Op&fsnotify.Remove != 0:
					if watchKind&protocol.WatchDelete != 0 {
						w.handleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Deleted))
					}
				case event.Op&fsnotify.Rename != 0:
					// For renames, first delete
					if watchKind&protocol.WatchDelete != 0 {
						w.handleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Deleted))
					}

					// Then check if the new file exists and create an event
					if info, err := os.Stat(event.Name); err == nil && !info.IsDir() {
						if watchKind&protocol.WatchCreate != 0 {
							w.debounceHandleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Created))
						}
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			watcherLogger.Error("Watcher error: %v", err)
		}
	}
}

// isPathWatched checks if a path should be watched based on server registrations
func (w *WorkspaceWatcher) isPathWatched(path string) (bool, protocol.WatchKind) {
	w.registrationMu.RLock()
	defer w.registrationMu.RUnlock()

	// If no explicit registrations, watch everything
	if len(w.registrations) == 0 {
		return true, protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
	}

	// Check each registration
	for _, reg := range w.registrations {
		isMatch := w.matchesPattern(path, reg.GlobPattern)
		if isMatch {
			kind := protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
			if reg.Kind != nil {
				kind = *reg.Kind
			}
			return true, kind
		}
	}

	return false, 0
}

// matchesGlob handles advanced glob patterns including ** and alternatives
func matchesGlob(pattern, path string) bool {
	// Handle file extension patterns with braces like *.{go,mod,sum}
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		// Extract extensions from pattern like "*.{go,mod,sum}"
		parts := strings.SplitN(pattern, "{", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			extPart := strings.SplitN(parts[1], "}", 2)
			if len(extPart) == 2 {
				extensions := strings.Split(extPart[0], ",")
				suffix := extPart[1]

				// Check if the path matches any of the extensions
				for _, ext := range extensions {
					extPattern := prefix + ext + suffix
					isMatch := matchesSimpleGlob(extPattern, path)
					if isMatch {
						return true
					}
				}
				return false
			}
		}
	}

	return matchesSimpleGlob(pattern, path)
}

// matchesSimpleGlob handles glob patterns with ** wildcards
func matchesSimpleGlob(pattern, path string) bool {
	// Handle special case for **/*.ext pattern (common in LSP)
	if strings.HasPrefix(pattern, "**/") {
		rest := strings.TrimPrefix(pattern, "**/")

		// If the rest is a simple file extension pattern like *.go
		if strings.HasPrefix(rest, "*.") {
			ext := strings.TrimPrefix(rest, "*")
			isMatch := strings.HasSuffix(path, ext)
			return isMatch
		}

		// Otherwise, try to check if the path ends with the rest part
		isMatch := strings.HasSuffix(path, rest)

		// If it matches directly, great!
		if isMatch {
			return true
		}

		// Otherwise, check if any path component matches
		pathComponents := strings.Split(path, "/")
		for i := range pathComponents {
			subPath := strings.Join(pathComponents[i:], "/")
			if strings.HasSuffix(subPath, rest) {
				return true
			}
		}

		return false
	}

	// Handle other ** wildcard pattern cases
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")

		// Validate the path starts with the first part
		if !strings.HasPrefix(path, parts[0]) && parts[0] != "" {
			return false
		}

		// For patterns like "**/*.go", just check the suffix
		if len(parts) == 2 && parts[0] == "" {
			isMatch := strings.HasSuffix(path, parts[1])
			return isMatch
		}

		// For other patterns, handle middle part
		remaining := strings.TrimPrefix(path, parts[0])
		if len(parts) == 2 {
			isMatch := strings.HasSuffix(remaining, parts[1])
			return isMatch
		}
	}

	// Handle simple * wildcard for file extension patterns (*.go, *.sum, etc)
	if strings.HasPrefix(pattern, "*.") {
		ext := strings.TrimPrefix(pattern, "*")
		isMatch := strings.HasSuffix(path, ext)
		return isMatch
	}

	// Fall back to simple matching for simpler patterns
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		watcherLogger.Error("Error matching pattern %s: %v", pattern, err)
		return false
	}

	return matched
}

// matchesPattern checks if a path matches the glob pattern
func (w *WorkspaceWatcher) matchesPattern(path string, pattern protocol.GlobPattern) bool {
	patternInfo, err := pattern.AsPattern()
	if err != nil {
		watcherLogger.Error("Error parsing pattern: %v", err)
		return false
	}

	basePath := patternInfo.GetBasePath()
	patternText := patternInfo.GetPattern()

	// watcherLogger.Debug("Matching path %s against pattern %s (base: %s)", path, patternText, basePath)

	path = filepath.ToSlash(path)

	// Special handling for wildcard patterns like "**/*"
	if patternText == "**/*" {
		// This should match any file
		// watcherLogger.Debug("Using special matching for **/* pattern")
		return true
	}

	// Special handling for wildcard patterns like "**/*.ext"
	if strings.HasPrefix(patternText, "**/") {
		if strings.HasPrefix(strings.TrimPrefix(patternText, "**/"), "*.") {
			// Extension pattern like **/*.go
			ext := strings.TrimPrefix(strings.TrimPrefix(patternText, "**/"), "*")
			// watcherLogger.Debug("Using extension matching for **/*.ext pattern: checking if %s ends with %s", path, ext)
			return strings.HasSuffix(path, ext)
		} else {
			// Any other pattern starting with **/ should match any path
			// watcherLogger.Debug("Using path substring matching for **/ pattern")
			return true
		}
	}

	// For simple patterns without base path
	if basePath == "" {
		// Check if the pattern matches the full path or just the file extension
		fullPathMatch := matchesGlob(patternText, path)
		baseNameMatch := matchesGlob(patternText, filepath.Base(path))

		watcherLogger.Debug("No base path, fullPathMatch: %v, baseNameMatch: %v", fullPathMatch, baseNameMatch)
		return fullPathMatch || baseNameMatch
	}

	// For relative patterns
	basePath = strings.TrimPrefix(basePath, "file://")
	basePath = filepath.ToSlash(basePath)

	// Make path relative to basePath for matching
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		watcherLogger.Error("Error getting relative path for %s: %v", path, err)
		return false
	}
	relPath = filepath.ToSlash(relPath)

	isMatch := matchesGlob(patternText, relPath)
	watcherLogger.Debug("Relative path matching: %s against %s = %v", relPath, patternText, isMatch)

	return isMatch
}

// debounceHandleFileEvent handles file events with debouncing to reduce notifications
func (w *WorkspaceWatcher) debounceHandleFileEvent(ctx context.Context, uri string, changeType protocol.FileChangeType) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Create a unique key based on URI and change type
	key := fmt.Sprintf("%s:%d", uri, changeType)

	// Cancel existing timer if any
	if timer, exists := w.debounceMap[key]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceMap[key] = time.AfterFunc(w.config.DebounceTime, func() {
		w.handleFileEvent(ctx, uri, changeType)

		// Cleanup timer after execution
		w.debounceMu.Lock()
		delete(w.debounceMap, key)
		w.debounceMu.Unlock()
	})
}

// handleFileEvent sends file change notifications
func (w *WorkspaceWatcher) handleFileEvent(ctx context.Context, uri string, changeType protocol.FileChangeType) {
	// If the file is open and it's a change event, use didChange notification
	filePath := uri[7:] // Remove "file://" prefix
	if changeType == protocol.FileChangeType(protocol.Changed) && w.client.IsFileOpen(filePath) {
		err := w.client.NotifyChange(ctx, filePath)
		if err != nil {
			watcherLogger.Error("Error notifying change: %v", err)
		}
		return
	}

	// Notify LSP server about the file event using didChangeWatchedFiles
	if err := w.notifyFileEvent(ctx, uri, changeType); err != nil {
		watcherLogger.Error("Error notifying LSP server about file event: %v", err)
	}
}

// notifyFileEvent sends a didChangeWatchedFiles notification for a file event
func (w *WorkspaceWatcher) notifyFileEvent(ctx context.Context, uri string, changeType protocol.FileChangeType) error {
	watcherLogger.Debug("Notifying file event: %s (type: %d)", uri, changeType)

	params := protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{
			{
				URI:  protocol.DocumentUri(uri),
				Type: changeType,
			},
		},
	}

	return w.client.DidChangeWatchedFiles(ctx, params)
}

// shouldExcludeDir returns true if the directory should be excluded from watching/opening
func (w *WorkspaceWatcher) shouldExcludeDir(dirPath string) bool {
	dirName := filepath.Base(dirPath)

	// Skip dot directories
	if strings.HasPrefix(dirName, ".") {
		return true
	}

	// Skip common excluded directories
	if w.config.ExcludedDirs[dirName] {
		return true
	}

	// Check gitignore patterns
	if w.gitignore != nil && w.gitignore.ShouldIgnore(dirPath, true) {
		watcherLogger.Debug("Directory %s excluded by gitignore pattern", dirPath)
		return true
	}

	return false
}

// shouldExcludeFile returns true if the file should be excluded from opening
func (w *WorkspaceWatcher) shouldExcludeFile(filePath string) bool {
	fileName := filepath.Base(filePath)

	// Skip dot files
	if strings.HasPrefix(fileName, ".") {
		return true
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if w.config.ExcludedFileExtensions[ext] || w.config.LargeBinaryExtensions[ext] {
		return true
	}

	// Skip temporary files
	if strings.HasSuffix(filePath, "~") {
		return true
	}

	// Check gitignore patterns
	if w.gitignore != nil && w.gitignore.ShouldIgnore(filePath, false) {
		watcherLogger.Debug("File %s excluded by gitignore pattern", filePath)
		return true
	}

	// Check file size
	info, err := os.Stat(filePath)
	if err != nil {
		// If we can't stat the file, skip it
		return true
	}

	// Skip large files
	if info.Size() > w.config.MaxFileSize {
		watcherLogger.Debug("Skipping large file: %s (%.2f MB)", filePath, float64(info.Size())/(1024*1024))
		return true
	}

	return false
}

// openMatchingFile opens a file if it matches any of the registered patterns
func (w *WorkspaceWatcher) openMatchingFile(ctx context.Context, path string) {
	// Skip directories
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}

	// Skip excluded files
	if w.shouldExcludeFile(path) {
		return
	}

	// Check if this path should be watched according to server registrations
	if watched, _ := w.isPathWatched(path); watched {
		// Don't need to check if it's already open - the client.OpenFile handles that
		if err := w.client.OpenFile(ctx, path); err != nil && watcherLogger.IsLevelEnabled(logging.LevelDebug) {
			watcherLogger.Debug("Error opening file %s: %v", path, err)
		}
	}
}
