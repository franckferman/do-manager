// Package sshkey wraps the DigitalOcean SSH Keys API.
package sshkey

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

// Service exposes SSH key operations against the DigitalOcean API.
type Service struct {
	client *godo.Client
}

// New returns a new SSH Key Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// List returns all SSH keys in the account.
func (s *Service) List(ctx context.Context) ([]godo.Key, error) {
	var all []godo.Key
	opts := &godo.ListOptions{PerPage: 200}

	for {
		keys, resp, err := s.client.Keys.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list ssh keys: %w", err)
		}
		all = append(all, keys...)
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}
		opts.Page = page + 1
	}

	return all, nil
}

// Create adds a new SSH public key to the account.
func (s *Service) Create(ctx context.Context, name, publicKey string) (*godo.Key, error) {
	req := &godo.KeyCreateRequest{
		Name:      name,
		PublicKey: publicKey,
	}
	key, _, err := s.client.Keys.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create ssh key: %w", err)
	}
	return key, nil
}

// Get retrieves a single SSH key by fingerprint.
func (s *Service) Get(ctx context.Context, fingerprint string) (*godo.Key, error) {
	key, _, err := s.client.Keys.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("get ssh key %s: %w", fingerprint, err)
	}
	return key, nil
}

// Delete removes an SSH key by fingerprint.
func (s *Service) Delete(ctx context.Context, fingerprint string) error {
	if _, err := s.client.Keys.DeleteByFingerprint(ctx, fingerprint); err != nil {
		return fmt.Errorf("delete ssh key %s: %w", fingerprint, err)
	}
	return nil
}
