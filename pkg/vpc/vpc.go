// Package vpc wraps the DigitalOcean VPCs API.
//
// # What is a VPC?
//
// A VPC (Virtual Private Cloud) is an isolated Layer-2 private network scoped
// to a single region. Every Droplet inside the VPC gets a private IP address
// in addition to its public one. Traffic between VPC members never leaves the
// DigitalOcean data-center fabric - it is not routed over the public internet.
//
// # Red Team relevance
//
//   - C2 server: no public port open for the C2 protocol, only SSH for admin.
//   - Redirector: sits in the same VPC, forwards HTTPS -> C2 via private IP.
//   - The C2 is unreachable from the internet directly; only the redirector
//     exposes ports 80/443.
//
// # IP range
//
// DO auto-assigns a /20 (4096 IPs) if you omit IPRange.
// You can supply any RFC-1918 CIDR, e.g. "10.20.0.0/24".
// The range must not overlap the DO hypervisor reserved ranges.
package vpc

import (
	"context"
	"fmt"
	"time"

	"github.com/digitalocean/godo"
)

// Service exposes VPC operations.
type Service struct {
	client *godo.Client
}

// New returns a new VPC Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// VPC is a simplified view of godo.VPC.
type VPC struct {
	ID          string
	Name        string
	Description string
	Region      string
	IPRange     string
	Default     bool
	CreatedAt   time.Time
}

func fromGodo(g *godo.VPC) VPC {
	return VPC{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		Region:      g.RegionSlug,
		IPRange:     g.IPRange,
		Default:     g.Default,
		CreatedAt:   g.CreatedAt,
	}
}

// Member is a resource (Droplet, Load Balancer, ...) attached to a VPC.
type Member struct {
	URN       string
	Name      string
	CreatedAt time.Time
}

// CreateOptions holds parameters for creating a VPC.
type CreateOptions struct {
	Name        string
	Region      string
	Description string
	// IPRange is optional. DO assigns a /20 automatically if empty.
	// Supply a CIDR like "10.20.0.0/24" if you want a specific range.
	IPRange string
}

// List returns all VPCs in the account.
func (s *Service) List(ctx context.Context) ([]VPC, error) {
	var all []VPC
	opts := &godo.ListOptions{PerPage: 200}
	for {
		vpcs, resp, err := s.client.VPCs.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list VPCs: %w", err)
		}
		for _, v := range vpcs {
			all = append(all, fromGodo(v))
		}
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

// Get returns a single VPC by ID.
func (s *Service) Get(ctx context.Context, id string) (VPC, error) {
	v, _, err := s.client.VPCs.Get(ctx, id)
	if err != nil {
		return VPC{}, fmt.Errorf("get VPC %q: %w", id, err)
	}
	return fromGodo(v), nil
}

// Create provisions a new VPC.
func (s *Service) Create(ctx context.Context, opts CreateOptions) (VPC, error) {
	req := &godo.VPCCreateRequest{
		Name:        opts.Name,
		RegionSlug:  opts.Region,
		Description: opts.Description,
		IPRange:     opts.IPRange,
	}
	v, _, err := s.client.VPCs.Create(ctx, req)
	if err != nil {
		return VPC{}, fmt.Errorf("create VPC: %w", err)
	}
	return fromGodo(v), nil
}

// Delete removes a VPC. The VPC must have no members.
func (s *Service) Delete(ctx context.Context, id string) error {
	_, err := s.client.VPCs.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete VPC %q: %w", id, err)
	}
	return nil
}

// Members returns all resources attached to the VPC.
// resourceType filters by type, e.g. "droplet". Pass "" for all.
func (s *Service) Members(ctx context.Context, id, resourceType string) ([]Member, error) {
	var all []Member
	req := &godo.VPCListMembersRequest{ResourceType: resourceType}
	opts := &godo.ListOptions{PerPage: 200}
	for {
		members, resp, err := s.client.VPCs.ListMembers(ctx, id, req, opts)
		if err != nil {
			return nil, fmt.Errorf("list VPC members for %q: %w", id, err)
		}
		for _, m := range members {
			all = append(all, Member{
				URN:       m.URN,
				Name:      m.Name,
				CreatedAt: m.CreatedAt,
			})
		}
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
