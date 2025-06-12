package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/spf13/pflag"
)

// based on the excellent post
// https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/

var httpdSocketPath string
var baseOutputDir string

func newServer(state *State) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/image-builder-composer/v2/compose", handleCompose(state))
	mux.Handle("/api/image-builder-composer/v2/composes/{id}", handleComposeStatus(state))
	mux.Handle("/api/image-builder-composer/v2/composes", handleComposes(state))
	return mux
}

func run(ctx context.Context, args []string, getenv func(string) string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	fs := pflag.NewFlagSet("osbuild-decomposer", 0)
	fs.StringVar(&baseOutputDir, "base-output-dir", "/var/lib/osbuild-composer/artifacts", "base output dir")
	fs.StringVar(&httpdSocketPath, "socket-path", "/run/osbuild-decomposer-httpd.sock", "socket path to listen on")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	httpServer := &http.Server{
		Handler:           newServer(NewState()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	sockListener, err := net.Listen("unix", httpdSocketPath)
	if err != nil {
		return fmt.Errorf("cannot listen on unix socket %s: %w", httpdSocketPath, err)
	}

	go func() {
		slog.Info("listening", "socket", sockListener.Addr().String())
		if err := httpServer.Serve(sockListener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "error listening and serving: %s\n", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("error shutting down http server", "error", err)
		}
	}()
	wg.Wait()

	// XXX: handle cleanup of incomplete builds

	return nil
}

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
