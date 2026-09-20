package s3

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type fakeCredentials struct {
	creds aws.Credentials
	err   error
}

func (f fakeCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return f.creds, f.err
}

func expiringCredentials(d time.Duration) fakeCredentials {
	return fakeCredentials{creds: aws.Credentials{
		AccessKeyID:     "ASIATESTTESTTESTTEST",
		SecretAccessKey: "secret",
		SessionToken:    "session-token",
		CanExpire:       true,
		Expires:         time.Now().Add(d),
	}}
}

func staticCredentials() fakeCredentials {
	return fakeCredentials{creds: aws.Credentials{
		AccessKeyID:     "AKIATESTTESTTESTTEST",
		SecretAccessKey: "secret",
	}}
}
