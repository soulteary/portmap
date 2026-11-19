// Copyright 2026 soulteary
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/stats"
	"github.com/soulteary/portmap/internal/telemetry"
	"github.com/soulteary/portmap/internal/web"
)

func validateStatsEndpoint(endpoint string) error {
	if endpoint == "" {
		return nil
	}
	_, _, err := telemetry.Endpoint(endpoint)
	if err != nil {
		return errors.New(i18n.T(i18n.KeyErrTelemetry, err))
	}
	return nil
}

func mergeStatsEndpoint(dst *string, endpoint string) error {
	if endpoint == "" {
		return nil
	}
	if *dst != "" && *dst != endpoint {
		return errors.New(i18n.T(i18n.KeyErrConflictingMultiOption, "stats_endpoint", *dst, endpoint))
	}
	*dst = endpoint
	return nil
}

func startReporter(ctx context.Context, endpoint string, events *stats.EventLog, providers ...stats.Provider) func() {
	if endpoint == "" {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&telemetry.Reporter{Endpoint: endpoint, Providers: providers, Events: events}).Run(ctx)
	}()
	return func() { cancel(); <-done }
}

func defaultStatsSocket() string {
	// A per-user private directory avoids sharing an ingest socket across users.
	return "unix://" + filepath.Join(os.TempDir(), "portmap-"+strconv.Itoa(os.Getuid()), "stats.sock")
}

func runStats(argv []string) error {
	fs := flag.NewFlagSet("portmap stats", flag.ContinueOnError)
	endpoint := fs.String("listen", defaultStatsSocket(), i18n.T(i18n.KeyFlagStatsListen))
	allowPublic := fs.Bool("allow-public", false, i18n.T(i18n.KeyFlagProxyAllowPublic))
	journal := fs.String("journal", "portmap-stats.journal", i18n.T(i18n.KeyFlagStatsJournal))
	httpAddr := fs.String("http-addr", "127.0.0.1:9080", i18n.T(i18n.KeyFlagWebAddr))
	fs.String("lang", "", i18n.T(i18n.KeyFlagLang, "en/zh/ja/ko/fr/de"))
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(i18n.T(i18n.KeyErrTelemetry, "unexpected positional arguments"))
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return serveStats(ctx, *endpoint, *journal, *httpAddr, *allowPublic)
}

func serveStats(ctx context.Context, endpoint, journal, httpAddr string, public ...bool) error {
	if endpoint == defaultStatsSocket() {
		_, path, _ := telemetry.Endpoint(endpoint)
		if err := prepareSocketDirectory(filepath.Dir(path)); err != nil {
			return err
		}
	}
	store, err := telemetry.OpenStore(journal)
	if err != nil {
		return errors.New(i18n.T(i18n.KeyErrTelemetry, err))
	}
	defer func() { _ = store.Close() }()
	ln, err := telemetry.ListenWithPublic(endpoint, len(public) > 0 && public[0])
	if err != nil {
		return errors.New(i18n.T(i18n.KeyErrTelemetry, err))
	}
	defer func() { _ = ln.Close() }()
	log.Printf(i18n.T(i18n.KeyLogCollectorStarted), ln.Addr(), journal, httpAddr)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	webWait, webErrors := startCollectorHTTP(ctx, cancel, httpAddr, store)
	err = telemetry.Serve(ctx, ln, store)
	cancel()
	webWait()
	if webErr := drainStatsErr(webErrors); webErr != nil {
		return webErr
	}
	return err
}

// historyHandler is mounted by the collector's query server.
func historyHandler(store *telemetry.Store, history bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.Records(history))
	}
}

func collectorHandler(store *telemetry.Store) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/sources", historyHandler(store, false))
	mux.Handle("/api/history", historyHandler(store, true))
	sh := (&stats.Server{Providers: []stats.Provider{store}}).Handler()
	mux.Handle("/stats", sh)
	mux.Handle("/metrics", sh)
	mux.Handle("/", (&web.Server{Providers: []stats.Provider{store}, Events: store.Events}).Handler())
	return mux
}

func startCollectorHTTP(ctx context.Context, cancel context.CancelFunc, addr string, store *telemetry.Store) (func(), <-chan error) {
	done := make(chan struct{})
	errs := make(chan error, 1)
	go func() {
		defer close(done)
		defer close(errs)
		if addr == "" {
			return
		}
		ln, err := telemetry.Listen("tcp://" + addr)
		if err != nil {
			errs <- err
			cancel()
			return
		}
		defer func() { _ = ln.Close() }()
		srv := &http.Server{Handler: collectorHandler(store), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
		stop := context.AfterFunc(ctx, func() { _ = srv.Close() })
		defer stop()
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) && ctx.Err() == nil {
			errs <- err
			cancel()
		}
	}()
	return func() { <-done }, errs
}

func prepareSocketDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 || !ok || stat.Uid != uint32(os.Getuid()) {
		return errors.New(i18n.T(i18n.KeyErrTelemetry, "default socket directory must be private and owned by the current user"))
	}
	return nil
}
