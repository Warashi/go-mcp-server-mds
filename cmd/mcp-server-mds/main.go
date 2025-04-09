package main

import (
	"context"
	"flag"
	"log"
	"os"

	mcpmds "github.com/Warashi/go-mcp-server-mds"
)

func main() {
	var path, name, description string
	flag.StringVar(&path, "path", ".", "path to the directory to serve")
	flag.StringVar(&name, "name", "mcp-server-mds", "name of the server")
	flag.StringVar(&description, "description", "Markdown Documents Server", "description of the server")
	flag.Parse()

	server, err := mcpmds.New(name, description, os.DirFS(path))
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if err := server.ServeStdio(context.Background()); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
