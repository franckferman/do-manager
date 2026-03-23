// Package firewall wraps the DigitalOcean Cloud Firewalls API.
package firewall

import (
	"context"
	"fmt"
	"strings"

	"github.com/digitalocean/godo"
)

// Service exposes firewall operations against the DigitalOcean API.
type Service struct {
	client *godo.Client
}

// New returns a new Firewall Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// RuleSpec is a human-friendly rule definition.
// Format: "proto:ports:addresses"
// Examples:
//   "tcp:443:any"           -> allow TCP 443 from anywhere
//   "tcp:22:203.0.113.1"    -> allow TCP 22 from a specific IP
//   "tcp:8080-8090:10.0.0.0/8" -> allow TCP range from CIDR
//   "icmp:0:any"            -> allow ICMP from anywhere
type RuleSpec struct {
	Protocol  string
	PortRange string
	Addresses []string // "any" expands to "0.0.0.0/0,::/0"
}

// ParseRuleSpec parses a "proto:ports:addr1,addr2" string into a RuleSpec.
func ParseRuleSpec(s string) (RuleSpec, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return RuleSpec{}, fmt.Errorf("invalid rule %q: expected proto:ports:addresses", s)
	}
	proto := strings.ToLower(parts[0])
	if proto != "tcp" && proto != "udp" && proto != "icmp" {
		return RuleSpec{}, fmt.Errorf("invalid protocol %q: must be tcp, udp, or icmp", proto)
	}
	addrs := parts[2]
	if strings.ToLower(addrs) == "any" {
		addrs = "0.0.0.0/0,::/0"
	}
	return RuleSpec{
		Protocol:  proto,
		PortRange: parts[1],
		Addresses: strings.Split(addrs, ","),
	}, nil
}

func (r RuleSpec) toInbound() godo.InboundRule {
	ports := r.PortRange
	if r.Protocol == "icmp" {
		ports = "0"
	}
	return godo.InboundRule{
		Protocol:  r.Protocol,
		PortRange: ports,
		Sources:   &godo.Sources{Addresses: r.Addresses},
	}
}

func (r RuleSpec) toOutbound() godo.OutboundRule {
	ports := r.PortRange
	if r.Protocol == "icmp" {
		ports = "0"
	}
	return godo.OutboundRule{
		Protocol:     r.Protocol,
		PortRange:    ports,
		Destinations: &godo.Destinations{Addresses: r.Addresses},
	}
}

// CreateOptions holds parameters for creating a firewall.
type CreateOptions struct {
	Name          string
	InboundRules  []RuleSpec
	OutboundRules []RuleSpec
	DropletIDs    []int
	Tags          []string
}

// List returns all firewalls in the account.
func (s *Service) List(ctx context.Context) ([]godo.Firewall, error) {
	var all []godo.Firewall
	opts := &godo.ListOptions{PerPage: 200}
	for {
		fws, resp, err := s.client.Firewalls.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list firewalls: %w", err)
		}
		all = append(all, fws...)
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

// Get returns a single firewall by ID.
func (s *Service) Get(ctx context.Context, id string) (*godo.Firewall, error) {
	fw, _, err := s.client.Firewalls.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get firewall %q: %w", id, err)
	}
	return fw, nil
}

// Create creates a new firewall.
func (s *Service) Create(ctx context.Context, opts CreateOptions) (*godo.Firewall, error) {
	req := &godo.FirewallRequest{
		Name:       opts.Name,
		DropletIDs: opts.DropletIDs,
		Tags:       opts.Tags,
	}
	for _, r := range opts.InboundRules {
		req.InboundRules = append(req.InboundRules, r.toInbound())
	}
	for _, r := range opts.OutboundRules {
		req.OutboundRules = append(req.OutboundRules, r.toOutbound())
	}
	fw, _, err := s.client.Firewalls.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create firewall: %w", err)
	}
	return fw, nil
}

// Delete deletes a firewall by ID.
func (s *Service) Delete(ctx context.Context, id string) error {
	_, err := s.client.Firewalls.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete firewall %q: %w", id, err)
	}
	return nil
}

// AttachDroplets adds Droplet IDs to an existing firewall.
func (s *Service) AttachDroplets(ctx context.Context, id string, dropletIDs ...int) error {
	_, err := s.client.Firewalls.AddDroplets(ctx, id, dropletIDs...)
	if err != nil {
		return fmt.Errorf("attach droplets to firewall %q: %w", id, err)
	}
	return nil
}

// DetachDroplets removes Droplet IDs from an existing firewall.
func (s *Service) DetachDroplets(ctx context.Context, id string, dropletIDs ...int) error {
	_, err := s.client.Firewalls.RemoveDroplets(ctx, id, dropletIDs...)
	if err != nil {
		return fmt.Errorf("detach droplets from firewall %q: %w", id, err)
	}
	return nil
}

// AddInboundRules appends inbound rules to an existing firewall.
func (s *Service) AddInboundRules(ctx context.Context, id string, rules []RuleSpec) error {
	req := &godo.FirewallRulesRequest{}
	for _, r := range rules {
		req.InboundRules = append(req.InboundRules, r.toInbound())
	}
	_, err := s.client.Firewalls.AddRules(ctx, id, req)
	if err != nil {
		return fmt.Errorf("add inbound rules to firewall %q: %w", id, err)
	}
	return nil
}

// AddOutboundRules appends outbound rules to an existing firewall.
func (s *Service) AddOutboundRules(ctx context.Context, id string, rules []RuleSpec) error {
	req := &godo.FirewallRulesRequest{}
	for _, r := range rules {
		req.OutboundRules = append(req.OutboundRules, r.toOutbound())
	}
	_, err := s.client.Firewalls.AddRules(ctx, id, req)
	if err != nil {
		return fmt.Errorf("add outbound rules to firewall %q: %w", id, err)
	}
	return nil
}

// ListByDroplet returns all firewalls attached to a given Droplet.
func (s *Service) ListByDroplet(ctx context.Context, dropletID int) ([]godo.Firewall, error) {
	var all []godo.Firewall
	opts := &godo.ListOptions{PerPage: 200}
	for {
		fws, resp, err := s.client.Firewalls.ListByDroplet(ctx, dropletID, opts)
		if err != nil {
			return nil, fmt.Errorf("list firewalls for droplet %d: %w", dropletID, err)
		}
		all = append(all, fws...)
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
