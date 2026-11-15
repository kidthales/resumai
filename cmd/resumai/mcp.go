package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/kidthales/resumai/internal/mcp"
	"github.com/kidthales/resumai/internal/render"
	"github.com/kidthales/resumai/internal/vault"
)

func runMCP(args []string, stdout, stderr io.Writer, globalVault string) int {
	return runMCPWithIO(args, os.Stdin, stdout, stderr, globalVault)
}

func runMCPWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer, globalVault string) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	var flagErrBuf bytes.Buffer
	fs.SetOutput(&flagErrBuf)

	vaultPath := fs.String("vault", globalVault, "Path to vault SQLite database")
	if err := fs.Parse(args); err != nil {
		if flagErrBuf.Len() > 0 {
			fmt.Fprint(stderr, render.RedactSecrets(flagErrBuf.String()))
		}
		return 1
	}

	targetPath := *vaultPath
	if targetPath == "" {
		var err error
		targetPath, err = vault.DefaultPath()
		if err != nil {
			fmt.Fprintln(stderr, render.RedactSecrets(fmt.Sprintf("error resolving default vault path: %v", err)))
			return 1
		}
	}

	v, err := vault.OpenPath(targetPath)
	if err != nil {
		fmt.Fprintln(stderr, render.RedactSecrets(fmt.Sprintf("error opening vault (%s): %v", targetPath, err)))
		return 1
	}
	defer v.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mat := mcp.NewLocalMaterializer(v)
	srv := mcp.NewServer(v, mcp.WithPacketMaterializer(mat))
	if err := srv.ServeStdio(ctx, stdin, stdout); err != nil && err != context.Canceled {
		fmt.Fprintln(stderr, render.RedactSecrets(fmt.Sprintf("mcp server error: %v", err)))
		return 1
	}
	return 0
}
