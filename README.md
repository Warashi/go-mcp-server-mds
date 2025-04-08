# go-mcp-server-mds

A Go implementation of a Model Context Protocol (MCP) server that serves markdown files from a filesystem.

## Overview

This server provides a way to expose markdown files through the Model Context Protocol, making them accessible as resources and providing tools to list and read markdown files. It supports markdown files with YAML or TOML frontmatter.

## Features

- Serve markdown files via MCP
- List all available markdown files with metadata
- Read individual markdown file contents
- Support for YAML and TOML frontmatter
- File system abstraction using `fs.FS`
- Resource management with URI-based access

## Installation

```bash
go get github.com/Warashi/go-mcp-server-mds
```

## Usage

Here's a basic example of how to create and use the MCP markdown server:

```go
package main

import (
    "os"

    mcpmds "github.com/Warashi/go-mcp-server-mds"
)

func main() {
    // Create a new server using the current directory
    server, err := mcpmds.New(
        "markdown-server",
        "A server that provides access to markdown files",
        os.DirFS("."),
    )
    if err != nil {
        panic(err)
    }

    // Start the server
    if err := server.ServeStdio(context.Background()); err != nil {
        panic(err)
    }
}
```

## Available Tools

### listMarkdownFiles

Lists all markdown files managed by the server. Returns metadata including:
- File path
- File size
- Parsed frontmatter (if available)

### readMarkdownFile

Reads a specific markdown file. Requires:
- `path`: The path to the markdown file

Returns:
- File path
- File size
- Parsed frontmatter
- Full file content

## Resource Access

Resources are accessible via `file://` URIs. Each markdown file is registered as a resource with:
- URI: `file://{path}`
- Name: Base filename
- Description: JSON-encoded frontmatter
- MimeType: `text/markdown`
- Size: File size in bytes

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
