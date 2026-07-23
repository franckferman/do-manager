package droplet

import "testing"

func TestLookupGPU(t *testing.T) {
	p, err := LookupGPU("H100") // case-insensitive
	if err != nil {
		t.Fatalf("LookupGPU(h100): %v", err)
	}
	if p.Size != "gpu-h100x1-80gb" {
		t.Errorf("size = %q, want gpu-h100x1-80gb", p.Size)
	}
	if p.Image != "gpu-h100x1-base" {
		t.Errorf("image = %q, want gpu-h100x1-base (AI/ML base)", p.Image)
	}
	if p.Hourly <= 0 {
		t.Errorf("hourly = %v, want > 0", p.Hourly)
	}
}

func TestLookupGPUUnknown(t *testing.T) {
	if _, err := LookupGPU("rtx9999"); err == nil {
		t.Error("expected error for unknown preset")
	}
}

func TestValidRegion(t *testing.T) {
	p, _ := LookupGPU("h100")
	if !p.ValidRegion("nyc2") {
		t.Error("nyc2 should be a valid GPU region")
	}
	if p.ValidRegion("nyc1") {
		t.Error("nyc1 has no GPU and must be rejected")
	}
	if p.DefaultRegion() != "nyc2" {
		t.Errorf("default region = %q, want nyc2", p.DefaultRegion())
	}
}

func TestGPUPresetsSorted(t *testing.T) {
	ps := GPUPresets()
	if len(ps) < 2 || ps[0] != "h100" {
		t.Errorf("presets = %v, want sorted starting with h100", ps)
	}
}

func TestLookupL40S(t *testing.T) {
	p, err := LookupGPU("l40s")
	if err != nil {
		t.Fatalf("LookupGPU(l40s): %v", err)
	}
	if p.Size != "gpu-l40sx1-48gb" {
		t.Errorf("size = %q, want gpu-l40sx1-48gb", p.Size)
	}
	if p.Image != "gpu-h100x1-base" { // shared single-GPU AI/ML image
		t.Errorf("image = %q, want gpu-h100x1-base", p.Image)
	}
}
