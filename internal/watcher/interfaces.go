package watcher

import (
	"context"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// LSPClient defines the minimal interface needed by the watcher
type LSPClient interface {
	// IsFileOpen checks if a file is already open in the editor
	IsFileOpen(path string) bool

	// OpenFile opens a file in the editor
	OpenFile(ctx context.Context, path string) error

	// NotifyChange notifies the server of a file change
	NotifyChange(ctx context.Context, path string) error

	// DidChangeWatchedFiles sends watched file events to the server
	DidChangeWatchedFiles(ctx context.Context, params protocol.DidChangeWatchedFilesParams) error
}

// WatcherConfig holds basic configuration for the watcher
type WatcherConfig struct {
	// DebounceTime is the duration to wait before sending file change events
	DebounceTime time.Duration

	// PreopenOnRegistration controls whether AddRegistrations should pre-open matching files.
	PreopenOnRegistration bool

	// PreopenMaxFiles limits file pre-opening per registration (0 = unlimited).
	PreopenMaxFiles int

	// ExcludedDirs are directory names that should be excluded from watching
	ExcludedDirs map[string]bool

	// ExcludedFileExtensions are file extensions that should be excluded from watching
	ExcludedFileExtensions map[string]bool

	// LargeBinaryExtensions are file extensions for large binary files that shouldn't be opened
	LargeBinaryExtensions map[string]bool

	// MaxFileSize is the maximum size of a file to open
	MaxFileSize int64
}

// DefaultWatcherConfig returns a configuration with sensible defaults
func DefaultWatcherConfig() *WatcherConfig {
	return &WatcherConfig{
		DebounceTime:          300 * time.Millisecond,
		PreopenOnRegistration: true,
		PreopenMaxFiles:       0,
		ExcludedDirs: map[string]bool{
			".git":         true,
			"node_modules": true,
			"dist":         true,
			"build":        true,
			"out":          true,
			"bin":          true,
			".idea":        true,
			".vscode":      true,
			".cache":       true,
			"coverage":     true,
			"target":       true, // Rust build output
			"vendor":       true, // Go vendor directory
		},
		ExcludedFileExtensions: map[string]bool{
			".swp":   true,
			".swo":   true,
			".tmp":   true,
			".temp":  true,
			".bak":   true,
			".log":   true,
			".o":     true, // Object files
			".so":    true, // Shared libraries
			".dylib": true, // macOS shared libraries
			".dll":   true, // Windows shared libraries
			".a":     true, // Static libraries
			".exe":   true, // Windows executables
			".lock":  true, // Lock files
		},
		LargeBinaryExtensions: map[string]bool{
			".png":  true,
			".jpg":  true,
			".jpeg": true,
			".gif":  true,
			".bmp":  true,
			".ico":  true,
			".zip":  true,
			".tar":  true,
			".gz":   true,
			".rar":  true,
			".7z":   true,
			".pdf":  true,
			".mp3":  true,
			".mp4":  true,
			".mov":  true,
			".wav":  true,
			".wasm": true,
		},
		MaxFileSize: 5 * 1024 * 1024, // 5MB
	}
}
