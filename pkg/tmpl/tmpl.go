// Package tmpl provides built-in cloud-init templates for common Red Team roles.
// Templates are bash scripts embedded in the binary via //go:embed.
// Variable substitution uses Go's text/template with {{.VarName}} syntax.
package tmpl

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

//go:embed scripts
var scriptsFS embed.FS

// Meta holds the parsed metadata from a template's comment header.
type Meta struct {
	Name string
	Desc string
	Vars []VarDef
}

// VarDef describes a single template variable.
type VarDef struct {
	Name    string
	Default string
	Desc    string
}

// List returns metadata for all available templates, sorted by filename.
func List() ([]Meta, error) {
	entries, err := fs.ReadDir(scriptsFS, "scripts")
	if err != nil {
		return nil, fmt.Errorf("read embedded scripts: %w", err)
	}
	var metas []Meta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		content, err := scriptsFS.ReadFile("scripts/" + e.Name())
		if err != nil {
			continue
		}
		metas = append(metas, parseMeta(content))
	}
	return metas, nil
}

// Raw returns the unrendered content of a template by name (without .sh extension).
func Raw(name string) (string, error) {
	content, err := scriptsFS.ReadFile("scripts/" + name + ".sh")
	if err != nil {
		return "", fmt.Errorf("template %q not found (run 'do-manager template list' to see available templates)", name)
	}
	return string(content), nil
}

// Render renders a named template with the provided variables.
// Unset variables fall back to their declared defaults from the script header.
func Render(name string, vars map[string]string) (string, error) {
	raw, err := Raw(name)
	if err != nil {
		return "", err
	}
	meta := parseMeta([]byte(raw))

	data := make(map[string]string)
	for _, v := range meta.Vars {
		data[v.Name] = v.Default
	}
	for k, v := range vars {
		data[k] = v
	}

	tmpl, err := template.New(name).Option("missingkey=zero").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template %q: %w", name, err)
	}
	return buf.String(), nil
}

// parseMeta extracts metadata from the leading comment block of a script.
// Recognized directives: @name, @desc, @var (KEY=default - description).
func parseMeta(content []byte) Meta {
	var m Meta
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			if m.Name != "" {
				break
			}
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		switch {
		case strings.HasPrefix(line, "@name:"):
			m.Name = strings.TrimSpace(strings.TrimPrefix(line, "@name:"))
		case strings.HasPrefix(line, "@desc:"):
			m.Desc = strings.TrimSpace(strings.TrimPrefix(line, "@desc:"))
		case strings.HasPrefix(line, "@var:"):
			rest := strings.TrimSpace(strings.TrimPrefix(line, "@var:"))
			var vd VarDef
			if idx := strings.Index(rest, " - "); idx != -1 {
				vd.Desc = rest[idx+3:]
				rest = rest[:idx]
			}
			kv := strings.SplitN(rest, "=", 2)
			vd.Name = kv[0]
			if len(kv) == 2 {
				vd.Default = kv[1]
			}
			m.Vars = append(m.Vars, vd)
		}
	}
	return m
}
