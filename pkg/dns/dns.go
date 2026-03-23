// Package dns wraps the DigitalOcean Domains and DNS Records API.
package dns

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

// Service exposes domain and DNS record operations.
type Service struct {
	client *godo.Client
}

// New returns a new DNS Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// ListDomains returns all domains in the account.
func (s *Service) ListDomains(ctx context.Context) ([]godo.Domain, error) {
	var all []godo.Domain
	opts := &godo.ListOptions{PerPage: 200}
	for {
		domains, resp, err := s.client.Domains.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list domains: %w", err)
		}
		all = append(all, domains...)
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

// GetDomain returns a single domain by name.
func (s *Service) GetDomain(ctx context.Context, name string) (*godo.Domain, error) {
	d, _, err := s.client.Domains.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get domain %q: %w", name, err)
	}
	return d, nil
}

// CreateDomain registers a new domain (optionally pointing to an IP).
func (s *Service) CreateDomain(ctx context.Context, name, ip string) (*godo.Domain, error) {
	req := &godo.DomainCreateRequest{Name: name}
	if ip != "" {
		req.IPAddress = ip
	}
	d, _, err := s.client.Domains.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create domain %q: %w", name, err)
	}
	return d, nil
}

// DeleteDomain deletes a domain and all its records.
func (s *Service) DeleteDomain(ctx context.Context, name string) error {
	_, err := s.client.Domains.Delete(ctx, name)
	if err != nil {
		return fmt.Errorf("delete domain %q: %w", name, err)
	}
	return nil
}

// ListRecords returns all DNS records for a domain.
func (s *Service) ListRecords(ctx context.Context, domain string) ([]godo.DomainRecord, error) {
	var all []godo.DomainRecord
	opts := &godo.ListOptions{PerPage: 200}
	for {
		records, resp, err := s.client.Domains.Records(ctx, domain, opts)
		if err != nil {
			return nil, fmt.Errorf("list records for %q: %w", domain, err)
		}
		all = append(all, records...)
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

// CreateRecord creates a DNS record for a domain.
// recordType: A, AAAA, CNAME, MX, TXT, NS, SRV, CAA
func (s *Service) CreateRecord(ctx context.Context, domain, recordType, name, data string, ttl int) (*godo.DomainRecord, error) {
	if ttl <= 0 {
		ttl = 1800
	}
	req := &godo.DomainRecordEditRequest{
		Type: recordType,
		Name: name,
		Data: data,
		TTL:  ttl,
	}
	r, _, err := s.client.Domains.CreateRecord(ctx, domain, req)
	if err != nil {
		return nil, fmt.Errorf("create %s record for %q: %w", recordType, domain, err)
	}
	return r, nil
}

// DeleteRecord deletes a DNS record by ID.
func (s *Service) DeleteRecord(ctx context.Context, domain string, recordID int) error {
	_, err := s.client.Domains.DeleteRecord(ctx, domain, recordID)
	if err != nil {
		return fmt.Errorf("delete record %d for %q: %w", recordID, domain, err)
	}
	return nil
}
