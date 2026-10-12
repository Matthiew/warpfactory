package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	catalogv1 "github.com/Matthiew/warpfactory/gen/catalog/v1"
	"github.com/Matthiew/warpfactory/internal/catalog"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "config.json", "path to JSON configuration")
	httpAddr := flag.String("http-addr", ":8080", "HTTP listen address")
	grpcAddr := flag.String("grpc-addr", ":9090", "gRPC listen address")
	flag.Parse()

	store, err := catalog.NewStore(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	httpListener, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatal(err)
	}
	grpcListener, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		httpListener.Close()
		log.Fatal(err)
	}

	service := catalog.NewService(store)
	httpServer := &http.Server{
		Handler:           service.HTTPHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServer(grpcServer, service)
	serveErrors := make(chan error, 2)
	go func() { serveErrors <- httpServer.Serve(httpListener) }()
	go func() { serveErrors <- grpcServer.Serve(grpcListener) }()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	log.Printf("HTTP listening on %s; gRPC listening on %s", httpListener.Addr(), grpcListener.Addr())
	var serveErr error

	for {
		select {
		case received := <-signals:
			if received == syscall.SIGHUP {
				if err := store.Reload(); err != nil {
					log.Printf("config reload failed (keeping last valid config): %v", err)
				} else {
					log.Print("config reloaded")
				}
				continue
			}
		case err := <-serveErrors:
			if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
				serveErr = err
			}
		}
		break
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-ctx.Done():
		grpcServer.Stop()
	}
	if serveErr != nil {
		log.Fatalf("server stopped: %v", serveErr)
	}
}
