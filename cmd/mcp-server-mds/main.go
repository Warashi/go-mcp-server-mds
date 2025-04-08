package main

import (
	"context"
	"flag"
	"log"
	"os"

	mcpmds "github.com/Warashi/go-mcp-server-mds"
)

func main() {
	var p string
	flag.StringVar(&p, "path", ".", "path to the directory to serve")
	flag.Parse()

	server, err := mcpmds.New("mcp-server-mds", "Markdown Documents Server", os.DirFS(p))
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if err := server.ServeStdio(context.Background()); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
