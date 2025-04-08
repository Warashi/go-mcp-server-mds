package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"iter"
	"net/url"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
	"github.com/Warashi/go-modelcontextprotocol/jsonschema"
	"github.com/Warashi/go-modelcontextprotocol/mcp"
	"github.com/goccy/go-yaml"
)

type server struct {
	name        string
	description string
	fs          fs.FS
	opts        []mcp.ServerOption
}

func NewServer(name, description string, fs fs.FS, opts ...mcp.ServerOption) (*mcp.Server, error) {
	s := &server{
		name:        name,
		description: description,
		fs:          fs,
		opts:        opts,
	}
	return s.server()
}

func (s *server) server() (*mcp.Server, error) {
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

func (s *server) listMarkdownFilesTool() mcp.Tool[*listMarkdownFilesRequest, *listMarkdownFilesResponse] {
	return mcp.NewToolFunc(
		"listMarkdownFiles",
		"List all markdown files managed by this server",
		jsonschema.Object{},
		s.listMarkdownFiles,
	)
}

type listMarkdownFilesRequest struct{}

type listMarkdownFilesResponse struct {
	Files []markdownFileInfo `json:"files"`
}

type markdownFileInfo struct {
	Path        string         `json:"path"`
	Size        int64          `json:"size"`
	Frontmatter map[string]any `json:"frontmatter"`
}

func (s *server) markdownFiles() iter.Seq[markdownFileInfo] {
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

func (s *server) listMarkdownFiles(ctx context.Context, _ *listMarkdownFilesRequest) (*listMarkdownFilesResponse, error) {
	return &listMarkdownFilesResponse{Files: slices.Collect(s.markdownFiles())}, nil
}

func (s *server) readMarkdownInfo(path string, d fs.DirEntry) (markdownFileInfo, error) {
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

func (s *server) readFrontmatter(content []byte) (map[string]any, error) {
	type unmarshaler struct {
		Unmarshaler func([]byte, interface{}) error
		Delimiter   string
	}
	unmarshalers := []unmarshaler{
		{yaml.Unmarshal, "---\n"},
		{toml.Unmarshal, "+++\n"},
	}

	for _, u := range unmarshalers {
		if bytes.HasPrefix(bytes.TrimSpace(content), []byte(u.Delimiter)) {
			start := bytes.Index(content, []byte(u.Delimiter))
			if start == -1 {
				continue
			}
			end := bytes.Index(content[start+len(u.Delimiter):], []byte("\n"+u.Delimiter))
			if end == -1 {
				continue
			}
			var frontmatter map[string]any
			if err := u.Unmarshaler(content[start+len(u.Delimiter):end], &frontmatter); err != nil {
				return nil, err
			}
			return frontmatter, nil
		}
	}
	return nil, nil
}

func (s *server) readMarkdownFileTool() mcp.Tool[*readMarkdownFileRequest, *readMarkdownFileResponse] {
	return mcp.NewToolFunc(
		"readMarkdownFile",
		"Read a markdown file",
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

type readMarkdownFileResponse struct {
	Path        string         `json:"path"`
	Size        int64          `json:"size"`
	Frontmatter map[string]any `json:"frontmatter"`
	Content     string         `json:"content"`
}

func (s *server) readMarkdownFile(ctx context.Context, request *readMarkdownFileRequest) (*readMarkdownFileResponse, error) {
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

func (s *server) listResourcesOption() ([]mcp.ServerOption, error) {
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

func (s *server) resourceReader() mcp.ResourceReader {
	return s
}

func (s *server) ReadResource(ctx context.Context, request *mcp.Request[mcp.ReadResourceRequestParams]) (*mcp.Result[mcp.ReadResourceResultData], error) {
	u, err := url.Parse(request.Params.URI)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "file" {
		return nil, errors.New("unsupported scheme: " + u.Scheme)
	}

	content, err := fs.ReadFile(s.fs, u.Path)
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
