package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/kidthales/resumai/internal/mcp"
	"github.com/kidthales/resumai/internal/vault"
)

func runMCP(args []string, stdout, stderr io.Writer, globalVault string) int {
	return runMCPWithIO(args, os.Stdin, stdout, stderr, globalVault)
}

func runMCPWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer, globalVault string) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)

	vaultPath := fs.String("vault", globalVault, "Path to vault SQLite database")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	targetPath := *vaultPath
	if targetPath == "" {
		var err error
		targetPath, err = vault.DefaultPath()
		if err != nil {
			fmt.Fprintf(stderr, "error resolving default vault path: %v\n", err)
			return 1
		}
	}

	v, err := vault.OpenPath(targetPath)
	if err != nil {
		fmt.Fprintf(stderr, "error opening vault (%s): %v\n", targetPath, err)
		return 1
	}
	defer v.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mat := mcp.NewLocalMaterializer(v)
	srv := mcp.NewServer(v, mcp.WithPacketMaterializer(mat))
	if err := srv.ServeStdio(ctx, stdin, stdout); err != nil && err != context.Canceled {
		fmt.Fprintf(stderr, "mcp server error: %v\n", err)
		return 1
	}
	return 0
}
