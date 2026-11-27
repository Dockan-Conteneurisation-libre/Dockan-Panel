// Command dockan-panel serves the Dockan Panel web interface.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockan-panel/internal/api"
	"dockan-panel/internal/auth"
	"dockan-panel/internal/config"
	"dockan-panel/internal/dockan"
	"dockan-panel/internal/runner"
	"dockan-panel/internal/stacks"
	"dockan-panel/internal/store"
	"dockan-panel/internal/terminal"
	"dockan-panel/internal/webui"
)

func main() {
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *version {
		fmt.Println(config.AppName, config.Version)
		return
	}
	if err := run(); err != nil {
		log.Fatalf("dockan-panel: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.EnsureStorage(); err != nil {
		return err
	}
	r := &runner.Runner{Bin: cfg.Dockan, PathPrefix: cfg.PathPrefix}
	srv := &api.Server{
		Cfg: cfg, R: r, D: dockan.Client{R: r},
		Users: auth.NewUsers(cfg.AuthFile()), Sessions: auth.NewSessions(), Rate: auth.NewRateLimiter(),
		Stacks: stacks.Store{Dir: cfg.StacksDir()},
		Store: &store.Store{R: r, Dir: cfg.StoreDir(), RootDir: cfg.StoreRoot(), StateFile: cfg.ImageStateFile(),
			Home: cfg.HomeDir, Root: runner.IsRoot()},
		Terms: terminal.NewManager(),
		UI:    webui.FS(),
	}
	addr := net.JoinHostPort(cfg.Bind, cfg.Port)
	httpSrv := api.NewHTTPServer(addr, srv.Handler())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		log.Printf("%s %s listening on http://%s (storage: %s)", config.AppName, config.Version, addr, cfg.Storage)
		errc <- httpSrv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdown); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
	}
	_ = os.Stdout.Sync() // flush logs before exit
	return nil
}
