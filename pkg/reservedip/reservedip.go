// Package reservedip wraps the DigitalOcean Reserved IPs API.
package reservedip

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

// Service exposes reserved IP operations against the DigitalOcean API.
type Service struct {
	client *godo.Client
}

// New returns a new Reserved IP Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// List returns all reserved IPs in the account.
func (s *Service) List(ctx context.Context) ([]godo.ReservedIP, error) {
	var all []godo.ReservedIP
	opts := &godo.ListOptions{PerPage: 200}
	for {
		ips, resp, err := s.client.ReservedIPs.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list reserved IPs: %w", err)
		}
		all = append(all, ips...)
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

// Get returns a single reserved IP by address.
func (s *Service) Get(ctx context.Context, ip string) (*godo.ReservedIP, error) {
	rip, _, err := s.client.ReservedIPs.Get(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("get reserved IP %q: %w", ip, err)
	}
	return rip, nil
}

// Reserve allocates a new reserved IP in the given region.
// Optionally assigns it to a Droplet immediately if dropletID > 0.
func (s *Service) Reserve(ctx context.Context, region string, dropletID int) (*godo.ReservedIP, error) {
	req := &godo.ReservedIPCreateRequest{Region: region}
	if dropletID > 0 {
		req.DropletID = dropletID
	}
	rip, _, err := s.client.ReservedIPs.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("reserve IP in %q: %w", region, err)
	}
	return rip, nil
}

// Delete releases a reserved IP back to the pool.
func (s *Service) Delete(ctx context.Context, ip string) error {
	_, err := s.client.ReservedIPs.Delete(ctx, ip)
	if err != nil {
		return fmt.Errorf("delete reserved IP %q: %w", ip, err)
	}
	return nil
}

// Assign binds a reserved IP to a Droplet.
func (s *Service) Assign(ctx context.Context, ip string, dropletID int) (*godo.Action, error) {
	action, _, err := s.client.ReservedIPActions.Assign(ctx, ip, dropletID)
	if err != nil {
		return nil, fmt.Errorf("assign %q to droplet %d: %w", ip, dropletID, err)
	}
	return action, nil
}

// Unassign detaches a reserved IP from its current Droplet.
func (s *Service) Unassign(ctx context.Context, ip string) (*godo.Action, error) {
	action, _, err := s.client.ReservedIPActions.Unassign(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("unassign reserved IP %q: %w", ip, err)
	}
	return action, nil
}
