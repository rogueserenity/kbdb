package main

// Config is populated from environment variables via Kong. There are no CLI
// flags for this service — it's a Lambda entrypoint, not a CLI tool — but
// Kong's struct-tag env binding, defaults, and required-field validation are
// useful regardless of whether flag parsing is ever exercised.
type Config struct {
	OIDCIssuerURL string `env:"OIDC_ISSUER_URL" required:""`
	OIDCAudience  string `env:"OIDC_AUDIENCE" required:""`
	// Rendered into the GET /authorize consent page - see internal/consent.
	StytchPublicToken  string `env:"STYTCH_PUBLIC_TOKEN" required:""`
	ImagesBucketName   string `env:"IMAGES_BUCKET_NAME" required:""`
	SwitchTableName    string `env:"SWITCH_TABLE_NAME" required:""`
	KeyboardTableName  string `env:"KEYBOARD_TABLE_NAME" required:""`
	KeycapSetTableName string `env:"KEYCAP_SET_TABLE_NAME" required:""`
	BuildTableName     string `env:"BUILD_TABLE_NAME" required:""`

	// Empty in real deployments; set locally to point at LocalStack.
	DynamoDBEndpointURL string `env:"DYNAMODB_ENDPOINT_URL"`

	// Empty in real deployments; set locally to point at LocalStack.
	S3EndpointURL string `env:"S3_ENDPOINT_URL"`
}
