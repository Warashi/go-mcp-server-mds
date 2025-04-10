// Package mcpmds provides a server implementation for the Model Context Protocol (MCP)
// that serves markdown files from a given filesystem.
package mcpmds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Warashi/go-modelcontextprotocol/jsonschema"
	"github.com/Warashi/go-modelcontextprotocol/mcp"
	"github.com/goccy/go-yaml"
)

// Server implements the core logic for serving markdown files via MCP.
// It wraps an fs.FS and provides tools and resource reading capabilities.
type Server struct {
	name               string
	description        string
	fs                 fs.FS
	opts               []mcp.ServerOption
	excludeFrontmatter []string
}

// ServerOption is a function that configures a Server.
type ServerOption func(*Server)

// WithMCPOptions sets additional MCP server options.
func WithMCPOptions(opts ...mcp.ServerOption) ServerOption {
	return func(s *Server) {
		s.opts = append(s.opts, opts...)
	}
}

// WithExcludeFrontmatter sets the frontmatter keys to exclude from the resource description.
func WithExcludeFrontmatter(keys ...string) ServerOption {
	return func(s *Server) {
		s.excludeFrontmatter = append(s.excludeFrontmatter, keys...)
	}
}

// New creates a new MCP server instance configured to serve markdown files from
// the provided filesystem.
// It initializes the server with a name, description, the filesystem, and optional
// mcp.ServerOption configurations.
func New(name, description string, fs fs.FS, opts ...ServerOption) (*mcp.Server, error) {
	s := &Server{
		name:        name,
		description: description,
		fs:          fs,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s.server()
}

func (s *Server) server() (*mcp.Server, error) {
	opts, err := s.listResourcesOption()
	if err != nil {
		return nil, err
	}
	opts = append(opts,
		mcp.WithResourceReader(s.resourceReader()),
		mcp.WithTool(s.listMarkdownFilesTool()),
		mcp.WithTool(s.readMarkdownFileTool()),
	)
	opts = append(opts, s.opts...)
	return mcp.NewServer(s.name, s.description, opts...)
}

func (s *Server) listMarkdownFilesTool() mcp.Tool[*listMarkdownFilesRequest, *listMarkdownFilesResponse] {
	return mcp.NewToolFunc(
		fmt.Sprintf("list_%s_markdown_files", s.name),
		fmt.Sprintf("List all markdown files managed by %s", s.name),
		jsonschema.Object{},
		s.listMarkdownFiles,
	)
}

type listMarkdownFilesRequest struct{}

type listMarkdownFilesResponse struct {
	Files []markdownFileInfo `json:"files"`
}

// markdownFileInfo holds metadata about a single markdown file.
type markdownFileInfo struct {
	// Path is the relative path to the markdown file within the server's filesystem.
	Path string `json:"path"`
	// Size is the size of the markdown file in bytes.
	Size int64 `json:"size"`
	// Frontmatter is a map containing the parsed frontmatter of the markdown file.
	// It can be nil if no frontmatter is found or parsable.
	Frontmatter map[string]any `json:"frontmatter"`
}

func (s *Server) markdownFiles() iter.Seq[markdownFileInfo] {
	return func(yield func(markdownFileInfo) bool) {
		fs.WalkDir(s.fs, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".md" {
				return nil
			}
			info, err := s.readMarkdownInfo(path, d)
			if err != nil {
				return err
			}
			if !yield(info) {
				return fs.SkipAll
			}
			return nil
		})
	}
}

func (s *Server) listMarkdownFiles(ctx context.Context, _ *listMarkdownFilesRequest) (*listMarkdownFilesResponse, error) {
	return &listMarkdownFilesResponse{Files: slices.Collect(s.markdownFiles())}, nil
}

func (s *Server) readMarkdownInfo(path string, d fs.DirEntry) (markdownFileInfo, error) {
	info, err := d.Info()
	if err != nil {
		return markdownFileInfo{}, err
	}
	content, err := fs.ReadFile(s.fs, path)
	if err != nil {
		return markdownFileInfo{}, err
	}
	frontmatter, err := s.readFrontmatter(content)
	if err != nil {
		return markdownFileInfo{}, err
	}
	return markdownFileInfo{
		Path:        path,
		Size:        info.Size(),
		Frontmatter: frontmatter,
	}, nil
}

func (s *Server) readFrontmatter(content []byte) (map[string]any, error) {
	type unmarshaler struct {
		Unmarshaler func([]byte, interface{}) error
		Delimiter   string
	}
	unmarshalers := []unmarshaler{
		{yaml.Unmarshal, "---\n"},
		{toml.Unmarshal, "+++\n"},
	}

	content = bytes.TrimSpace(content)
	for _, u := range unmarshalers {
		if bytes.HasPrefix(content, []byte(u.Delimiter)) {
			start := bytes.Index(content, []byte(u.Delimiter))
			if start == -1 {
				continue
			}
			end := bytes.Index(content[start+len(u.Delimiter):], []byte("\n"+u.Delimiter))
			if end == -1 {
				continue
			}
			var frontmatter map[string]any
			if err := u.Unmarshaler(content[start+len(u.Delimiter):start+len(u.Delimiter)+end], &frontmatter); err != nil {
				return nil, err
			}
			for _, key := range s.excludeFrontmatter {
				delete(frontmatter, key)
			}
			if len(frontmatter) == 0 {
				return nil, nil
			}
			return frontmatter, nil
		}
	}
	return nil, nil
}

func (s *Server) readMarkdownFileTool() mcp.Tool[*readMarkdownFileRequest, *readMarkdownFileResponse] {
	return mcp.NewToolFunc(
		fmt.Sprintf("read_%s_markdown_file", s.name),
		fmt.Sprintf("Read a markdown file managed by %s", s.name),
		jsonschema.Object{
			Properties: map[string]jsonschema.Schema{
				"path": jsonschema.String{
					Description: "The path to the markdown file",
				},
			},
			Required: []string{"path"},
		},
		s.readMarkdownFile,
	)
}

type readMarkdownFileRequest struct {
	Path string `json:"path" jsonschema:"required"`
}

// readMarkdownFileResponse defines the response structure for the readMarkdownFile tool.
// It includes the file's metadata and its full content.
type readMarkdownFileResponse struct {
	// Path is the relative path to the markdown file.
	Path string `json:"path"`
	// Size is the size of the markdown file in bytes.
	Size int64 `json:"size"`
	// Frontmatter contains the parsed frontmatter data.
	Frontmatter map[string]any `json:"frontmatter"`
	// Content is the full text content of the markdown file.
	Content string `json:"content"`
}

func (s *Server) readMarkdownFile(ctx context.Context, request *readMarkdownFileRequest) (*readMarkdownFileResponse, error) {
	content, err := fs.ReadFile(s.fs, request.Path)
	if err != nil {
		return nil, err
	}
	info, err := fs.Stat(s.fs, request.Path)
	if err != nil {
		return nil, err
	}
	frontmatter, err := s.readFrontmatter(content)
	if err != nil {
		return nil, err
	}
	return &readMarkdownFileResponse{
		Path:        request.Path,
		Size:        info.Size(),
		Frontmatter: frontmatter,
		Content:     string(content),
	}, nil
}

func (s *Server) listResourcesOption() ([]mcp.ServerOption, error) {
	opts := []mcp.ServerOption{}
	for f := range s.markdownFiles() {
		desc, err := json.Marshal(f.Frontmatter)
		if err != nil {
			return nil, err
		}
		opts = append(opts, mcp.WithResource(mcp.Resource{
			URI:         "file://" + f.Path,
			Name:        filepath.Base(f.Path),
			Description: string(desc),
			MimeType:    "text/markdown",
			Size:        f.Size,
		}))
	}
	return opts, nil
}

func (s *Server) resourceReader() mcp.ResourceReader {
	return s
}

// ReadResource implements the mcp.ResourceReader interface.
// It reads the content of a resource specified by a file URI.
func (s *Server) ReadResource(ctx context.Context, request *mcp.Request[mcp.ReadResourceRequestParams]) (*mcp.Result[mcp.ReadResourceResultData], error) {
	if !strings.HasPrefix(request.Params.URI, "file://") {
		return nil, errors.New("unsupported scheme: " + request.Params.URI)
	}

	content, err := fs.ReadFile(s.fs, request.Params.URI[7:])
	if err != nil {
		return nil, err
	}

	return &mcp.Result[mcp.ReadResourceResultData]{
		Data: mcp.ReadResourceResultData{
			Contents: []mcp.IsResourceContents{
				mcp.TextResourceContents{
					URI:      request.Params.URI,
					Text:     string(content),
					MimeType: "text/markdown",
				},
			},
		},
	}, nil
}
