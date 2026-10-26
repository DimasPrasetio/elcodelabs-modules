package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"testbed-app/internal/config"
	"testbed-app/internal/database"
	"testbed-app/internal/kernel"
	"testbed-app/internal/modules"
	"testbed-app/internal/server"
	"testbed-app/internal/shared/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	// .env lives at the repo root; the API runs from apps/api. Errors are ignored:
	// in the container, env is injected via env_file.
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load(".env")

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.Env)

	db, err := database.Open(cfg.DBDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	auth := &kernel.AuthSlot{}
	deps := &kernel.Deps{
		Config: cfg, DB: db, Log: log, Auth: auth,
		Authz: kernel.NewAuthzSlot(auth),
		Jobs:  kernel.NewJobsSlot(db, database.Name(cfg.DBDSN), log),
		Audit: kernel.NewAuditSlot(auth, log),
	}
	srv, err := server.New(deps, func(mux *http.ServeMux) error { return modules.Register(mux, deps) })
	if err != nil {
		return err
	}

	// docker stop sends SIGTERM: finish in-flight requests before exiting.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Background jobs start once every module has registered its own. Their context is not the
	// signal's: on SIGTERM requests drain first, then deps.Jobs.Stop cancels the jobs.
	deps.Jobs.Start(context.Background())

	errc := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", srv.Addr, "env", cfg.Env)
		errc <- srv.ListenAndServe()
	}()

	// 25 s fits inside docker's stop_grace_period (30 s in the prod compose) and lets most
	// long requests finish; the server's WriteTimeout bounds the rest. Requests stop first, then
	// jobs — on every way out — then (deferred) the database.
	stopJobs := func(ctx context.Context) {
		if err := deps.Jobs.Stop(ctx); err != nil {
			log.Warn("api: " + err.Error())
		}
	}

	select {
	case err := <-errc:
		jobsCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stopJobs(jobsCtx)
		return err
	case <-ctx.Done():
	}

	shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdown)
	stopJobs(shutdown)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("api stopped")
	return nil
}
