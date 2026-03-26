// Package snapshot wraps the DigitalOcean Snapshots API.
package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/digitalocean/godo"
)

const actionPollInterval = 5 * time.Second
const actionTimeout = 10 * time.Minute

// Service exposes snapshot operations against the DigitalOcean API.
type Service struct {
	client *godo.Client
}

// New returns a new Snapshot Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// List returns all Droplet snapshots in the account.
func (s *Service) List(ctx context.Context) ([]godo.Snapshot, error) {
	var all []godo.Snapshot
	opts := &godo.ListOptions{PerPage: 200}
	for {
		snaps, resp, err := s.client.Snapshots.ListDroplet(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list snapshots: %w", err)
		}
		all = append(all, snaps...)
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

// Get returns a single snapshot by ID.
func (s *Service) Get(ctx context.Context, id string) (*godo.Snapshot, error) {
	snap, _, err := s.client.Snapshots.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get snapshot %q: %w", id, err)
	}
	return snap, nil
}

// Delete deletes a snapshot by ID.
func (s *Service) Delete(ctx context.Context, id string) error {
	_, err := s.client.Snapshots.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete snapshot %q: %w", id, err)
	}
	return nil
}

// CreateFromDroplet initiates a snapshot of the given Droplet.
// If wait is true, it polls until the action completes or ctx is cancelled.
// Returns the triggered Action (use ID to track progress).
func (s *Service) CreateFromDroplet(ctx context.Context, dropletID int, name string, wait bool) (*godo.Action, error) {
	action, _, err := s.client.DropletActions.Snapshot(ctx, dropletID, name)
	if err != nil {
		return nil, fmt.Errorf("snapshot droplet %d: %w", dropletID, err)
	}
	if !wait {
		return action, nil
	}
	if err := s.pollAction(ctx, dropletID, action.ID); err != nil {
		return action, err
	}
	return action, nil
}

// pollAction polls a droplet action until it completes or ctx expires.
func (s *Service) pollAction(ctx context.Context, dropletID, actionID int) error {
	ticker := time.NewTicker(actionPollInterval)
	defer ticker.Stop()
	deadline := time.After(actionTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for action %d to complete", actionID)
		case <-ticker.C:
			action, _, err := s.client.DropletActions.Get(ctx, dropletID, actionID)
			if err != nil {
				return fmt.Errorf("poll action %d: %w", actionID, err)
			}
			switch action.Status {
			case "completed":
				return nil
			case "errored":
				return fmt.Errorf("action %d errored", actionID)
			}
		}
	}
}
