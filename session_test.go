package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSessionFileSerialization(t *testing.T) {
	tests := []struct {
		name        string                        // Test case description
		session     *LSPSession                   // Session to test
		checkFields func(*testing.T, *LSPSession) // Custom assertion function
	}{
		{
			name: "Empty session",
			session: &LSPSession{
				LSPs: []LSPSessionEntry{},
			},
			checkFields: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 0 {
					t.Errorf("Expected empty LSPs list")
				}
			},
		},
		{
			name: "Single LSP session",
			session: &LSPSession{
				LSPs: []LSPSessionEntry{
					{
						Workspace: "/path/to/project",
						Language:  "go",
					},
				},
			},
			checkFields: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 1 {
					t.Errorf("Expected 1 LSP entry")
				}
				if s.LSPs[0].Workspace != "/path/to/project" {
					t.Errorf("Expected workspace /path/to/project")
				}
				if s.LSPs[0].Language != "go" {
					t.Errorf("Expected language go")
				}
			},
		},
		{
			name: "Multiple LSPs session",
			session: &LSPSession{
				LSPs: []LSPSessionEntry{
					{
						Workspace: "/backend",
						Language:  "go",
					},
					{
						Workspace: "/frontend",
						Language:  "typescript",
					},
					{
						Workspace: "/core",
						Language:  "rust",
					},
				},
			},
			checkFields: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 3 {
					t.Errorf("Expected 3 LSP entries, got %d", len(s.LSPs))
				}
				expected := map[string]string{
					"/backend":  "go",
					"/frontend": "typescript",
					"/core":     "rust",
				}
				for _, entry := range s.LSPs {
					if lang, ok := expected[entry.Workspace]; !ok || lang != entry.Language {
						t.Errorf("Unexpected LSP entry: %s -> %s", entry.Workspace, entry.Language)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFields(t, tt.session)
		})
	}
}

func TestSaveAndLoadSessionFile(t *testing.T) {
	tests := []struct {
		name          string                        // Test case description
		sessionToSave *LSPSession                   // Session to save
		expectSaveErr bool                          // Whether save error is expected
		expectLoadErr bool                          // Whether load error is expected
		checkLoaded   func(*testing.T, *LSPSession) // Custom assertion function
	}{
		{
			name: "Save and load valid session",
			sessionToSave: &LSPSession{
				LSPs: []LSPSessionEntry{
					{
						Workspace: "/project/backend",
						Language:  "go",
					},
					{
						Workspace: "/project/frontend",
						Language:  "typescript",
					},
				},
			},
			expectSaveErr: false,
			expectLoadErr: false,
			checkLoaded: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 2 {
					t.Errorf("Expected 2 LSP entries, got %d", len(s.LSPs))
				}
				if s.LSPs[0].Language != "go" {
					t.Errorf("Expected first LSP to be go")
				}
				if s.LSPs[1].Language != "typescript" {
					t.Errorf("Expected second LSP to be typescript")
				}
			},
		},
		{
			name: "Save and load empty session",
			sessionToSave: &LSPSession{
				LSPs: []LSPSessionEntry{},
			},
			expectSaveErr: false,
			expectLoadErr: false,
			checkLoaded: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 0 {
					t.Errorf("Expected empty LSPs list")
				}
			},
		},
		{
			name: "Save and load session with special characters",
			sessionToSave: &LSPSession{
				LSPs: []LSPSessionEntry{
					{
						Workspace: "/home/user/projects/my-project_v2",
						Language:  "go",
					},
				},
			},
			expectSaveErr: false,
			expectLoadErr: false,
			checkLoaded: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 1 {
					t.Errorf("Expected 1 LSP entry")
				}
				if s.LSPs[0].Workspace != "/home/user/projects/my-project_v2" {
					t.Errorf("Workspace with special characters not preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpfile, err := os.CreateTemp("", "session-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			tmpfile.Close()
			defer os.Remove(tmpfile.Name())

			// Save session
			err = SaveSessionFile(tmpfile.Name(), tt.sessionToSave)
			if tt.expectSaveErr {
				if err == nil {
					t.Errorf("Expected save error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected save error: %v", err)
				return
			}

			// Load session
			loaded, err := LoadSessionFile(tmpfile.Name())
			if tt.expectLoadErr {
				if err == nil {
					t.Errorf("Expected load error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected load error: %v", err)
				return
			}

			tt.checkLoaded(t, loaded)
		})
	}
}

func TestLoadSessionFileErrors(t *testing.T) {
	tests := []struct {
		name        string                  // Test case description
		filename    string                  // File to load
		expectError bool                    // Whether error is expected
		checkError  func(*testing.T, error) // Custom error assertion
	}{
		{
			name:        "Load nonexistent file",
			filename:    "/nonexistent/path/to/session.json",
			expectError: true,
			checkError: func(t *testing.T, err error) {
				if err == nil {
					t.Errorf("Expected error for nonexistent file")
				}
			},
		},
		{
			name:        "Load invalid JSON",
			filename:    "", // Will be created with invalid content
			expectError: true,
			checkError: func(t *testing.T, err error) {
				if err == nil {
					t.Errorf("Expected error for invalid JSON")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filename string
			if tt.filename != "" {
				filename = tt.filename
			} else {
				// Create temp file with invalid JSON
				tmpfile, err := os.CreateTemp("", "invalid-*.json")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				tmpfile.WriteString("{invalid json}")
				tmpfile.Close()
				defer os.Remove(tmpfile.Name())
				filename = tmpfile.Name()
			}

			_, err := LoadSessionFile(filename)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				tt.checkError(t, err)
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSessionFileFormat(t *testing.T) {
	tests := []struct {
		name         string                        // Test case description
		jsonData     string                        // JSON content to test
		expectErr    bool                          // Whether error is expected
		checkSession func(*testing.T, *LSPSession) // Custom assertion function
	}{
		{
			name: "Valid session JSON format",
			jsonData: `{
				"lsps": [
					{"workspace": "/path/to/workspace", "language": "go"},
					{"workspace": "/path/to/other", "language": "rust"}
				]
			}`,
			expectErr: false,
			checkSession: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 2 {
					t.Errorf("Expected 2 LSP entries")
				}
			},
		},
		{
			name: "Empty LSPs array",
			jsonData: `{
				"lsps": []
			}`,
			expectErr: false,
			checkSession: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 0 {
					t.Errorf("Expected empty LSPs array")
				}
			},
		},
		{
			name:      "Invalid JSON",
			jsonData:  "{invalid}",
			expectErr: true,
		},
		{
			name: "Missing workspace field",
			jsonData: `{
				"lsps": [
					{"language": "go"}
				]
			}`,
			expectErr: false, // JSON unmarshals, but workspace will be empty
			checkSession: func(t *testing.T, s *LSPSession) {
				if len(s.LSPs) != 1 {
					t.Errorf("Expected 1 LSP entry")
				}
				if s.LSPs[0].Workspace != "" {
					t.Errorf("Expected empty workspace")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpfile, err := os.CreateTemp("", "session-format-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			tmpfile.WriteString(tt.jsonData)
			tmpfile.Close()
			defer os.Remove(tmpfile.Name())

			session, err := LoadSessionFile(tmpfile.Name())
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.checkSession != nil {
					tt.checkSession(t, session)
				}
			}
		})
	}
}

func TestSessionMarshaling(t *testing.T) {
	session := &LSPSession{
		LSPs: []LSPSessionEntry{
			{
				Workspace: "/project",
				Language:  "go",
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(session)
	if err != nil {
		t.Errorf("Failed to marshal session: %v", err)
	}

	// Unmarshal back
	var unmarshaled LSPSession
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal session: %v", err)
	}

	if unmarshaled.LSPs[0].Workspace != "/project" {
		t.Errorf("Workspace not preserved in marshaling")
	}
	if unmarshaled.LSPs[0].Language != "go" {
		t.Errorf("Language not preserved in marshaling")
	}
}
