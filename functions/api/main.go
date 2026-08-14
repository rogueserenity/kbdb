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
	"time"

	"github.com/alecthomas/kong"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogueserenity/kbdb/internal/auth"
	"github.com/rogueserenity/kbdb/internal/repository/dynamo"
	imagestore "github.com/rogueserenity/kbdb/internal/repository/s3"
	"github.com/rogueserenity/kbdb/internal/router"
)

// Version is set at build time via -ldflags "-X main.Version=...", derived
// from `git describe --tags --always --dirty` (see functions/api/Dockerfile).
// Defaults to "dev" when built without that flag (e.g. `go build` directly).
var Version = "dev"

// port must match AWS_LWA_PORT in template.yaml; not 8080 since sam local
// start-api's Runtime Interface Emulator already binds that port in-container
// (github.com/aws/aws-lambda-web-adapter#125).
const port = "8000"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var cfg Config
	kong.Parse(&cfg)

	ctx := context.Background()
	verifier, err := auth.NewVerifier(ctx, cfg.OIDCIssuerURL, cfg.OIDCAudience)
	if err != nil {
		log.Fatalf("initializing token verifier: %v", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("loading AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		if cfg.DynamoDBEndpointURL != "" {
			o.BaseEndpoint = &cfg.DynamoDBEndpointURL
		}
	})
	switchRepo := dynamo.NewSwitchRepository(dynamoClient, cfg.SwitchTableName)
	keyboardRepo := dynamo.NewKeyboardRepository(dynamoClient, cfg.KeyboardTableName)
	keycapSetRepo := dynamo.NewKeycapSetRepository(dynamoClient, cfg.KeycapSetTableName)
	buildRepo := dynamo.NewBuildRepository(dynamoClient, cfg.BuildTableName)

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3EndpointURL != "" {
			o.BaseEndpoint = &cfg.S3EndpointURL
			// LocalStack needs path-style addressing (bucket.s3.amazonaws.com
			// virtual-hosted-style doesn't resolve against a non-AWS
			// endpoint); real S3 doesn't.
			o.UsePathStyle = true
		}
	})
	// Both entities' images currently live in the same bucket.
	presignClient := s3.NewPresignClient(s3Client)
	keycapKitImageStore := imagestore.NewKeycapKitImageStore(s3Client, presignClient, cfg.ImagesBucketName)
	buildImageStore := imagestore.NewBuildImageStore(s3Client, presignClient, cfg.ImagesBucketName)

	handler := router.New(verifier, switchRepo, keyboardRepo, keycapSetRepo, keycapKitImageStore, buildRepo, buildImageStore, cfg.OIDCIssuerURL, Version)

	// ReadHeaderTimeout bounds a slow/malicious client independently of
	// Lambda's own per-invocation timeout.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting server", "port", port, "version", Version)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
