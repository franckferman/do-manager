package cmd

import "strings"

import "testing"

func TestInjectTTLEmpty(t *testing.T) {
	out := injectTTL("", 60, "tok")
	if !strings.HasPrefix(out, "#!/bin/bash") {
		t.Errorf("want shebang, got %q", out[:20])
	}
	if !strings.Contains(out, "3600") { // 60 min * 60
		t.Error("expected sleep seconds (3600)")
	}
	if !strings.Contains(out, "-X DELETE") || !strings.Contains(out, "metadata/v1/id") {
		t.Error("expected self-DELETE via metadata id")
	}
}

func TestInjectTTLAfterShebang(t *testing.T) {
	script := "#!/bin/bash\nset -e\napt-get update\n"
	out := injectTTL(script, 30, "tok")
	lines := strings.Split(out, "\n")
	if lines[0] != "#!/bin/bash" {
		t.Errorf("shebang must stay first, got %q", lines[0])
	}
	// dead-man scheduled BEFORE the original body (set -e / apt)
	if strings.Index(out, "dead-man") > strings.Index(out, "apt-get") {
		t.Error("dead-man must be scheduled before the main script body")
	}
	if !strings.Contains(out, "1800") { // 30*60
		t.Error("expected 1800s sleep")
	}
}

func TestInjectTTLNoShebang(t *testing.T) {
	out := injectTTL("apt-get update\n", 10, "tok")
	if !strings.HasPrefix(out, "#!/bin/bash") {
		t.Error("should prepend a shebang when missing")
	}
}
