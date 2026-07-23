package droplet

import (
	"fmt"
	"sort"
	"strings"
)

// GPUPlan is a preset for a DigitalOcean GPU Droplet. GPU Droplets aren't just a
// bigger size: they need an AI/ML base image (NVIDIA drivers + CUDA preinstalled
// — required for GPU workloads like hashcat), they only exist in a few regions,
// and they are expensive. A preset fills all three and surfaces the hourly cost.
type GPUPlan struct {
	Model   string
	Size    string   // DO size slug
	Image   string   // AI/ML base image slug (drivers preinstalled)
	Hourly  float64  // approx USD/hr (DO pricing changes 2026-08-01; confirm current)
	Regions []string // regions that offer this plan (first = preferred default)
}

// DO single-GPU Droplets all use the gpu-h100x1-base image (per DO docs), even
// for non-H100 single-GPU plans. Slugs are DO-controlled — override --image /
// --region if DO changes them.
var gpuRegions = []string{"nyc2", "tor1", "atl1", "ams3"}

var gpuPlans = map[string]GPUPlan{
	"h100":   {"h100", "gpu-h100x1-80gb", "gpu-h100x1-base", 3.39, gpuRegions},
	"h100x8": {"h100x8", "gpu-h100x8-640gb", "gpu-h100x8-base", 23.92, gpuRegions},
	// L40S: cheaper single GPU, plenty for hashcat. Slug verified; single-GPU image
	// is the shared gpu-h100x1-base. (RTX 6000/4000 Ada slugs unverified -> not added.)
	"l40s": {"l40s", "gpu-l40sx1-48gb", "gpu-h100x1-base", 1.57, gpuRegions},
}

// GPUPresets returns the known preset names, sorted.
func GPUPresets() []string {
	names := make([]string, 0, len(gpuPlans))
	for k := range gpuPlans {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// LookupGPU resolves a preset name (case-insensitive) to its plan.
func LookupGPU(model string) (GPUPlan, error) {
	p, ok := gpuPlans[strings.ToLower(model)]
	if !ok {
		return GPUPlan{}, fmt.Errorf("unknown GPU preset %q (known: %s)",
			model, strings.Join(GPUPresets(), ", "))
	}
	return p, nil
}

// ValidRegion reports whether region offers this GPU plan.
func (p GPUPlan) ValidRegion(region string) bool {
	for _, r := range p.Regions {
		if r == region {
			return true
		}
	}
	return false
}

// DefaultRegion returns the preferred region for the plan.
func (p GPUPlan) DefaultRegion() string {
	if len(p.Regions) > 0 {
		return p.Regions[0]
	}
	return ""
}
