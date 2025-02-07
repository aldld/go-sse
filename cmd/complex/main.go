package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/tmaxmax/go-sse"
)

var sseHandler = sse.NewServer()

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("/stop", func(w http.ResponseWriter, _ *http.Request) {
		cancel()
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/", SnapshotHTTPEndpoint)
	mux.Handle("/events", sseHandler)

	s := &http.Server{
		Addr:              "0.0.0.0:8080",
		Handler:           cors(mux),
		ReadHeaderTimeout: time.Second * 10,
	}
	s.RegisterOnShutdown(func() {
		e := &sse.Message{}
		e.SetName("close")
		// Broadcast a close message so clients can gracefully disconnect.
		_ = sseHandler.Publish(e)
		_ = sseHandler.Shutdown()
	})

	go recordMetric(ctx, "ops", time.Second*2)
	go recordMetric(ctx, "cycles", time.Millisecond*500)

	go func() {
		duration := func() time.Duration {
			return time.Duration(2000+rand.Intn(1000)) * time.Millisecond
		}

		timer := time.NewTimer(duration())
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				_ = sseHandler.Publish(generateRandomNumbers())
			case <-ctx.Done():
				return
			}

			timer.Reset(duration())
		}
	}()

	if err := runServer(ctx, s); err != nil {
		log.Println("server closed", err)
	}
}

func recordMetric(ctx context.Context, metric string, frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			v := Inc(metric)

			e := &sse.Message{}
			e.SetTTL(frequency)
			e.SetName(metric)
			e.AppendData(strconv.AppendInt(nil, v, 10))

			_ = sseHandler.Publish(e)
		case <-ctx.Done():
			return
		}
	}
}

func runServer(ctx context.Context, s *http.Server) error {
	shutdownError := make(chan error)

	go func() {
		<-ctx.Done()

		sctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		shutdownError <- s.Shutdown(sctx)
	}()

	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-shutdownError
}

func generateRandomNumbers() *sse.Message {
	e := &sse.Message{}
	count := 1 + rand.Intn(5)

	for i := 0; i < count; i++ {
		e.AppendData(strconv.AppendUint(nil, rand.Uint64(), 10))
	}

	return e
}
