// Package client provides an authenticated DigitalOcean API client.
package client

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

// tokenSource implements oauth2.TokenSource for static bearer tokens.
type tokenSource struct {
	token string
}

func (t *tokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: t.token}, nil
}

// New creates an authenticated *godo.Client ready to use.
// Returns an error if token is empty.
func New(token string) (*godo.Client, error) {
	if token == "" {
		return nil, fmt.Errorf("API token must not be empty")
	}
	ts := &tokenSource{token: token}
	oauthClient := oauth2.NewClient(context.Background(), ts)
	return godo.NewClient(oauthClient), nil
}
