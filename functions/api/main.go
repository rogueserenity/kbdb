// Command api is the kbdb API server. It is a plain net/http server with no
// Lambda-specific code — in production it runs behind the aws-lambda-web-adapter
// extension, which translates API Gateway events into real HTTP requests
// against this process.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/alecthomas/kong"

	"github.com/rogueserenity/kbdb/internal/auth"
	"github.com/rogueserenity/kbdb/internal/router"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var cfg Config
	kong.Parse(&cfg)

	ctx := context.Background()
	verifier, err := auth.NewVerifier(ctx, cfg.OIDCIssuerURL, cfg.OIDCAudience)
	if err != nil {
		log.Fatalf("initializing token verifier: %v", err)
	}

	handler := router.New(verifier)

	slog.Info("starting server", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
