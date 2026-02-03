package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"lunar-lander/internal/server"
	"lunar-lander/internal/sugardb"
	"lunar-lander/internal/watch"
)

type runtimeState struct {
	mu      sync.RWMutex
	runtime *server.Runtime
}

func (s *runtimeState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.runtime
	current.Router.ServeHTTP(w, r)
}

func (s *runtimeState) Swap(next *server.Runtime) {
	s.mu.Lock()
	old := s.runtime
	s.runtime = next
	s.mu.Unlock()

	if old != nil {
		old.Engine.Close()
	}
}

func main() {
	var scriptPath string
	var addr string
	var watchEnabled bool
	flag.StringVar(&scriptPath, "script", "app.lua", "path to Lua script")
	flag.StringVar(&addr, "addr", ":8080", "address to listen on")
	flag.BoolVar(&watchEnabled, "watch", false, "watch the script path for changes")
	flag.Parse()

	state := &runtimeState{}
	store := sugardb.NewStore()
	initial, err := server.BuildRuntime(scriptPath, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		os.Exit(1)
	}
	state.runtime = initial

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if watchEnabled {
		watcher := watch.NewScriptWatcher(scriptPath, func() {
			log.Printf("script change detected: reloading %s", scriptPath)
			next, err := server.BuildRuntime(scriptPath, store)
			if err != nil {
				log.Printf("reload failed: %v", err)
				return
			}
			state.Swap(next)
		})
		go func() {
			if err := watcher.Run(ctx); err != nil {
				log.Printf("watcher stopped: %v", err)
			}
		}()
	}

	server := &http.Server{
		Addr:    addr,
		Handler: state,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
