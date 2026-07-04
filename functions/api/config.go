package main

// Config is populated from environment variables via Kong. There are no CLI
// flags for this service — it's a Lambda entrypoint, not a CLI tool — but
// Kong's struct-tag env binding, defaults, and required-field validation are
// useful regardless of whether flag parsing is ever exercised.
type Config struct {
	OIDCIssuerURL string `env:"OIDC_ISSUER_URL" required:""`
	OIDCAudience  string `env:"OIDC_AUDIENCE" required:""`
	// Not 8080: the Runtime Interface Emulator sam local start-api uses
	// binds port 8080 for its own Lambda Runtime API inside the container,
	// which collides with the app trying to listen on the same port in the
	// same container network namespace (github.com/aws/aws-lambda-web-adapter#125).
	// AWS_LWA_PORT is set to match in template.yaml so this isn't just a
	// local-dev-only override - the deployed adapter and app must agree
	// regardless of environment.
	Port string `env:"PORT" default:"8000"`
}
