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

## Frontmatter Support

The server supports markdown files with YAML or TOML frontmatter. Frontmatter is metadata placed at the beginning of a markdown file, enclosed by delimiters.

### YAML Frontmatter

YAML frontmatter uses `---` as delimiters:

```
---
title: My Document
date: 2024-03-21
tags: [documentation, markdown]
---

# Document Content
```

### TOML Frontmatter

TOML frontmatter uses `+++` as delimiters:

```
+++
title = "My Document"
date = 2024-03-21
tags = ["documentation", "markdown"]
+++

# Document Content
```

The frontmatter metadata is parsed and made available through the server's tools and resource descriptions. This metadata can include any valid YAML or TOML data and is useful for organizing and describing your markdown documents.

## Installation

```bash
go get github.com/Warashi/go-mcp-server-mds
```

## Usage

Here's a basic example of how to create and use the MCP markdown server:

```go
package main

import (
    "context"
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

## Command-Line Tool (`mcp-server-mds`)

This repository includes a command-line tool `mcp-server-mds` that runs the server directly.

### Installing

To build the command-line tool:

```bash
go install github.com/Warashi/go-mcp-server-mds/cmd/mcp-server-mds@latest
```

### Running

To run the server, execute the built binary. It serves markdown files from a specified directory over standard input/output.

```bash
$HOME/go/bin/mcp-server-mds -path /path/to/your/markdown/files
```

Flags:
- `-path`: Specifies the directory containing the markdown files to serve. Defaults to the current directory (`.`).
- `-name`: Sets the server name. Defaults to `mcp-server-mds`.
- `-description`: Sets the server description. Defaults to `Markdown Documents Server`.

## Available Tools

### list_{server-name}_markdown_files

Lists all markdown files managed by the server. Returns metadata including:
- File path
- File size
- Parsed frontmatter (if available)

### read_{server-name}_markdown_file

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
