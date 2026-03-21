// Package region wraps the DigitalOcean Regions, Sizes, and Images APIs.
package region

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

// Service exposes region/size/image queries against the DigitalOcean API.
type Service struct {
	client *godo.Client
}

// New returns a new Region Service.
func New(client *godo.Client) *Service {
	return &Service{client: client}
}

// ListRegions returns all available DigitalOcean regions.
func (s *Service) ListRegions(ctx context.Context) ([]godo.Region, error) {
	var all []godo.Region
	opts := &godo.ListOptions{PerPage: 200}

	for {
		regions, resp, err := s.client.Regions.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list regions: %w", err)
		}
		all = append(all, regions...)
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

// ListSizes returns all available Droplet size slugs.
func (s *Service) ListSizes(ctx context.Context) ([]godo.Size, error) {
	var all []godo.Size
	opts := &godo.ListOptions{PerPage: 200}

	for {
		sizes, resp, err := s.client.Sizes.List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list sizes: %w", err)
		}
		all = append(all, sizes...)
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

// ListImages returns available images filtered by type.
// Accepted values: "distribution", "application", "user", "" (all public images).
func (s *Service) ListImages(ctx context.Context, imageType string) ([]godo.Image, error) {
	// godo v1.109+ exposes dedicated list methods per image type
	// rather than a generic ImageListOptions struct.
	type listFn func(context.Context, *godo.ListOptions) ([]godo.Image, *godo.Response, error)

	var fn listFn
	switch imageType {
	case "distribution":
		fn = s.client.Images.ListDistribution
	case "application":
		fn = s.client.Images.ListApplication
	case "user":
		fn = s.client.Images.ListUser
	default:
		fn = s.client.Images.List
	}

	var all []godo.Image
	opts := &godo.ListOptions{PerPage: 200}

	for {
		images, resp, err := fn(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list images (%s): %w", imageType, err)
		}
		all = append(all, images...)
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
