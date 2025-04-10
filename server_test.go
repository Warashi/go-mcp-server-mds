package mcpmds

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Warashi/go-modelcontextprotocol/mcp"
)

func Test_server_readFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    map[string]any
		wantErr bool
	}{
		{
			name: "YAML frontmatter",
			content: []byte(`---
title: Test YAML
value: 123
---
Regular content`),
			want: map[string]any{
				"title": "Test YAML",
				"value": uint64(123),
			},
			wantErr: false,
		},
		{
			name: "TOML frontmatter",
			content: []byte(`+++
title = "Test TOML"
value = 456
+++
Regular content`),
			want: map[string]any{
				"title": "Test TOML",
				"value": int64(456), // TOML decoder uses int64
			},
			wantErr: false,
		},
		{
			name: "YAML frontmatter with extra whitespace",
			content: []byte(`

---
title: Test YAML Whitespace
---


Regular content`),
			want: map[string]any{
				"title": "Test YAML Whitespace",
			},
			wantErr: false,
		},
		{
			name:    "No frontmatter",
			content: []byte(`Just regular content`),
			want:    nil,
			wantErr: false,
		},
		{
			name:    "Empty content",
			content: []byte{},
			want:    nil,
			wantErr: false,
		},
		{
			name: "Invalid YAML",
			content: []byte(`---
title: Test Invalid YAML
value: [1, 2
---
Regular content`),
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid TOML",
			content: []byte(`+++
title = "Test Invalid TOML"
value = "unterminated string
+++
Regular content`),
			want:    nil,
			wantErr: true,
		},
		{
			name: "Delimiter inside content (YAML)",
			content: []byte(`---
title: Test YAML
---
Content with --- delimiter`),
			want: map[string]any{
				"title": "Test YAML",
			},
			wantErr: false,
		},
		{
			name: "Delimiter inside content (TOML)",
			content: []byte(`+++
title = "Test TOML"
+++
Content with +++ delimiter`),
			want: map[string]any{
				"title": "Test TOML",
			},
			wantErr: false,
		},
		{
			name: "Only delimiter (YAML)",
			content: []byte(`---
---`),
			want:    nil,
			wantErr: false,
		},
		{
			name: "Only delimiter (TOML)",
			content: []byte(`+++
+++`),
			want:    nil,
			wantErr: false,
		},
	}

	s := &Server{} // Create a dummy server instance, fs is not needed for this method

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.readFrontmatter(tt.content)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(tt.want, got) {
					t.Errorf("readFrontmatter() got = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	testFS := fstest.MapFS{
		"file1.md": {Data: []byte("content1")},
		"dir/file2.md": {Data: []byte(`---
title: File 2
---
content2`)},
		"not_markdown.txt": {Data: []byte("text")},
	}

	// Test that New runs without error for a valid FS
	srv, err := New("test-server", "test description", testFS)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil server")
	}

	// We cannot easily inspect the name, description, or resources
	// from the returned mcp.Server without exporting internal details
	// or specific methods in the mcp library.
	// Testing for non-nil return and no error is the primary goal here.
}

func Test_server_listMarkdownFiles(t *testing.T) {
	now := time.Now()
	testFS := fstest.MapFS{
		"file1.md":         {Data: []byte("content1"), ModTime: now, Mode: 0644},
		"dir/file2.md":     {Data: []byte("---\ntitle: File 2\n---\ncontent2"), ModTime: now, Mode: 0644},
		"dir/subdir/f3.md": {Data: []byte("content3"), ModTime: now, Mode: 0644},
		"skip.txt":         {Data: []byte("text"), ModTime: now, Mode: 0644},
		"another.md":       {Data: []byte("content4"), ModTime: now, Mode: 0644},
		// Add a file without read permissions (though fstest might not fully enforce this)
		"noread.md": {Data: []byte("cannot read"), ModTime: now, Mode: 0000},
		// Add a directory named like a markdown file
		"fake.md": {Mode: fs.ModeDir | 0755, ModTime: now},
	}

	s := &Server{fs: testFS} // Only fs is needed for listMarkdownFiles

	resp, err := s.listMarkdownFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("listMarkdownFiles() error = %v", err)
	}

	wantFiles := []markdownFileInfo{
		{
			Path:        "another.md",
			Size:        int64(len(testFS["another.md"].Data)),
			Frontmatter: nil,
		},
		{
			Path:        "dir/file2.md",
			Size:        int64(len(testFS["dir/file2.md"].Data)),
			Frontmatter: map[string]any{"title": "File 2"},
		},
		{
			Path:        "dir/subdir/f3.md",
			Size:        int64(len(testFS["dir/subdir/f3.md"].Data)),
			Frontmatter: nil,
		},
		{
			Path:        "file1.md",
			Size:        int64(len(testFS["file1.md"].Data)),
			Frontmatter: nil,
		},
		{
			Path:        "noread.md", // Expect it to be listed even if content read might fail elsewhere
			Size:        int64(len(testFS["noread.md"].Data)),
			Frontmatter: nil,
		},
	}

	// Sort both slices for consistent comparison
	slices.SortFunc(resp.Files, func(a, b markdownFileInfo) int {
		return strings.Compare(a.Path, b.Path)
	})
	slices.SortFunc(wantFiles, func(a, b markdownFileInfo) int {
		return strings.Compare(a.Path, b.Path)
	})

	if !reflect.DeepEqual(resp.Files, wantFiles) {
		t.Errorf("listMarkdownFiles()\n got = %+v,\nwant = %+v", resp.Files, wantFiles)
	}
}

func Test_server_readMarkdownFile(t *testing.T) {
	now := time.Now()
	testFS := fstest.MapFS{
		"file1.md":          {Data: []byte("content1"), ModTime: now, Mode: 0644},
		"dir/file2.md":      {Data: []byte("---\ntitle: File 2\n---\ncontent2"), ModTime: now, Mode: 0644},
		"empty.md":          {Data: []byte(""), ModTime: now, Mode: 0644},
		"no_frontmatter.md": {Data: []byte("just content"), ModTime: now, Mode: 0644},
	}

	s := &Server{fs: testFS}

	tests := []struct {
		name    string
		path    string
		want    *readMarkdownFileResponse
		wantErr bool
	}{
		{
			name: "Read file with frontmatter",
			path: "dir/file2.md",
			want: &readMarkdownFileResponse{
				Path:        "dir/file2.md",
				Size:        int64(len(testFS["dir/file2.md"].Data)),
				Frontmatter: map[string]any{"title": "File 2"},
				Content:     "---\ntitle: File 2\n---\ncontent2",
			},
			wantErr: false,
		},
		{
			name: "Read file without frontmatter",
			path: "no_frontmatter.md",
			want: &readMarkdownFileResponse{
				Path:        "no_frontmatter.md",
				Size:        int64(len(testFS["no_frontmatter.md"].Data)),
				Frontmatter: nil,
				Content:     "just content",
			},
			wantErr: false,
		},
		{
			name: "Read empty file",
			path: "empty.md",
			want: &readMarkdownFileResponse{
				Path:        "empty.md",
				Size:        0,
				Frontmatter: nil,
				Content:     "",
			},
			wantErr: false,
		},
		{
			name:    "Read non-existent file",
			path:    "not/a/file.md",
			want:    nil,
			wantErr: true, // Expect fs.ErrNotExist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &readMarkdownFileRequest{Path: tt.path}
			got, err := s.readMarkdownFile(context.Background(), req)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				// Optionally check for specific error like fs.ErrNotExist
				if tt.path == "not/a/file.md" && !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("expected fs.ErrNotExist, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("readMarkdownFile()\n got = %+v,\nwant = %+v", got, tt.want)
				}
			}
		})
	}
}

func Test_server_ReadResource(t *testing.T) {
	now := time.Now()
	testFS := fstest.MapFS{
		"file1.md":     {Data: []byte("content1"), ModTime: now, Mode: 0644},
		"dir/file2.md": {Data: []byte("content2"), ModTime: now, Mode: 0644},
	}

	s := &Server{fs: testFS}

	tests := []struct {
		name    string
		uri     string
		want    *mcp.Result[mcp.ReadResourceResultData]
		wantErr bool
	}{
		{
			name: "Read valid file URI",
			uri:  "file://file1.md",
			want: &mcp.Result[mcp.ReadResourceResultData]{
				Data: mcp.ReadResourceResultData{
					Contents: []mcp.IsResourceContents{
						mcp.TextResourceContents{
							URI:      "file://file1.md",
							Text:     "content1",
							MimeType: "text/markdown",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Read valid file URI in subdirectory",
			uri:  "file://dir/file2.md",
			want: &mcp.Result[mcp.ReadResourceResultData]{
				Data: mcp.ReadResourceResultData{
					Contents: []mcp.IsResourceContents{
						mcp.TextResourceContents{
							URI:      "file://dir/file2.md",
							Text:     "content2",
							MimeType: "text/markdown",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "Read non-existent file URI",
			uri:     "file://nonexistent.md",
			want:    nil,
			wantErr: true, // Expect fs.ErrNotExist
		},
		{
			name:    "Unsupported scheme",
			uri:     "http://example.com/file.md",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid URI",
			uri:     ":invalid:",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &mcp.Request[mcp.ReadResourceRequestParams]{
				Params: mcp.ReadResourceRequestParams{URI: tt.uri},
			}
			got, err := s.ReadResource(context.Background(), req)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				// Keep specific error checks where useful
				if strings.Contains(tt.name, "non-existent") && !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("expected fs.ErrNotExist, got %v", err)
				}
				if strings.Contains(tt.name, "Unsupported scheme") && !strings.Contains(err.Error(), "unsupported scheme") {
					t.Errorf("expected 'unsupported scheme' error, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ReadResource() got = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}
