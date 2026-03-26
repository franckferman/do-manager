// Package droplet wraps the DigitalOcean Droplets API.
// It can be imported independently of the CLI.
package droplet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/digitalocean/godo"
)

// Service exposes Droplet operations against the DigitalOcean API.
type Service struct {
	client *godo.Client
}

// New returns a new Droplet Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// CreateOptions holds all parameters for provisioning a Droplet.
type CreateOptions struct {
	Name     string
	Region   string
	Size     string
	Image    string
	SSHKeys  []int
	Tags     []string
	UserData string
	IPv6     bool
	Backups  bool
	// VPCUUID places the Droplet inside a specific VPC.
	// Leave empty to use the region's default VPC.
	VPCUUID string
}

// Create provisions a new Droplet and returns the created resource.
func (s *Service) Create(ctx context.Context, opts CreateOptions) (*godo.Droplet, error) {
	keys := make([]godo.DropletCreateSSHKey, len(opts.SSHKeys))
	for i, k := range opts.SSHKeys {
		keys[i] = godo.DropletCreateSSHKey{ID: k}
	}

	req := &godo.DropletCreateRequest{
		Name:     opts.Name,
		Region:   opts.Region,
		Size:     opts.Size,
		Image:    godo.DropletCreateImage{Slug: opts.Image},
		SSHKeys:  keys,
		Tags:     opts.Tags,
		UserData: opts.UserData,
		IPv6:     opts.IPv6,
		Backups:  opts.Backups,
		VPCUUID:  opts.VPCUUID,
	}

	d, _, err := s.client.Droplets.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create droplet: %w", err)
	}
	return d, nil
}

// List returns all Droplets in the account, handling pagination transparently.
func (s *Service) List(ctx context.Context) ([]godo.Droplet, error) {
	var all []godo.Droplet
	opts := &godo.ListOptions{PerPage: 200}

	for {
		droplets, resp, err := s.client.Droplets.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list droplets: %w", err)
		}
		all = append(all, droplets...)
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

// Get returns a single Droplet by ID.
func (s *Service) Get(ctx context.Context, id int) (*godo.Droplet, error) {
	d, _, err := s.client.Droplets.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get droplet %d: %w", id, err)
	}
	return d, nil
}

// Delete destroys a Droplet by ID.
func (s *Service) Delete(ctx context.Context, id int) error {
	if _, err := s.client.Droplets.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete droplet %d: %w", id, err)
	}
	return nil
}

// PowerOn powers on a Droplet.
func (s *Service) PowerOn(ctx context.Context, id int) error {
	if _, _, err := s.client.DropletActions.PowerOn(ctx, id); err != nil {
		return fmt.Errorf("power on droplet %d: %w", id, err)
	}
	return nil
}

// PowerOff gracefully powers off a Droplet.
func (s *Service) PowerOff(ctx context.Context, id int) error {
	if _, _, err := s.client.DropletActions.PowerOff(ctx, id); err != nil {
		return fmt.Errorf("power off droplet %d: %w", id, err)
	}
	return nil
}

// Reboot reboots a Droplet.
func (s *Service) Reboot(ctx context.Context, id int) error {
	if _, _, err := s.client.DropletActions.Reboot(ctx, id); err != nil {
		return fmt.Errorf("reboot droplet %d: %w", id, err)
	}
	return nil
}

// BatchResult holds the outcome of a single CreateBatch slot.
type BatchResult struct {
	Name    string
	Droplet *godo.Droplet
	Err     error
}

// CreateBatch provisions count Droplets in parallel.
// If count > 1 the names are suffixed: name-01, name-02, ...
// Each goroutine writes to its own index (no mutex needed).
// Panics inside goroutines are recovered and surfaced as errors.
func (s *Service) CreateBatch(ctx context.Context, opts CreateOptions, count int) []BatchResult {
	results := make([]BatchResult, count)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					results[idx].Err = fmt.Errorf("internal panic: %v", r)
				}
			}()
			o := opts
			if count > 1 {
				o.Name = fmt.Sprintf("%s-%02d", opts.Name, idx+1)
			}
			results[idx].Name = o.Name
			d, err := s.Create(ctx, o)
			results[idx].Droplet = d
			results[idx].Err = err
		}(i)
	}

	wg.Wait()
	return results
}

// DeleteMany destroys multiple Droplets in parallel and returns all errors (indexed to match ids).
// Panics inside goroutines are recovered and surfaced as errors.
func (s *Service) DeleteMany(ctx context.Context, ids []int) []error {
	errs := make([]error, len(ids))
	var wg sync.WaitGroup

	for i, id := range ids {
		wg.Add(1)
		go func(idx, dropletID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errs[idx] = fmt.Errorf("internal panic: %v", r)
				}
			}()
			errs[idx] = s.Delete(ctx, dropletID)
		}(i, id)
	}

	wg.Wait()
	return errs
}

// DeleteByTag destroys all Droplets carrying the given tag in a single API call.
func (s *Service) DeleteByTag(ctx context.Context, tag string) error {
	if _, err := s.client.Droplets.DeleteByTag(ctx, tag); err != nil {
		return fmt.Errorf("delete droplets by tag %q: %w", tag, err)
	}
	return nil
}

// Rebuild re-images a Droplet from a new image slug or ID string.
// The Droplet keeps its ID, reserved IPs, tags, and firewall assignments.
// Returns the triggered Action; the caller can poll it or ignore it.
//
// Why use Rebuild instead of delete + create?
//
//   - The Droplet ID stays the same (firewall rules, monitoring remain attached)
//   - Reserved IPs stay assigned automatically
//   - Much faster than reprovisioning from scratch
//   - Useful when a node is compromised or burned: wipe and start clean
func (s *Service) Rebuild(ctx context.Context, id int, imageSlug string) (*godo.Action, error) {
	action, _, err := s.client.DropletActions.RebuildByImageSlug(ctx, id, imageSlug)
	if err != nil {
		return nil, fmt.Errorf("rebuild droplet %d with image %q: %w", id, imageSlug, err)
	}
	return action, nil
}

// WaitRebuild polls a droplet action until completed or ctx expires.
func (s *Service) WaitRebuild(ctx context.Context, dropletID, actionID int) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			action, _, err := s.client.DropletActions.Get(ctx, dropletID, actionID)
			if err != nil {
				return fmt.Errorf("poll rebuild action %d: %w", actionID, err)
			}
			switch action.Status {
			case "completed":
				return nil
			case "errored":
				return fmt.Errorf("rebuild action %d errored", actionID)
			}
		}
	}
}

// PublicIPv4 extracts the public IPv4 address of a Droplet, or an empty string.
func PublicIPv4(d *godo.Droplet) string {
	ip, _ := d.PublicIPv4()
	return ip
}

// PublicIPv6 extracts the public IPv6 address of a Droplet, or an empty string.
func PublicIPv6(d *godo.Droplet) string {
	ip, _ := d.PublicIPv6()
	return ip
}
