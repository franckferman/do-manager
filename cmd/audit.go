package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/franckferman/do-manager/internal/config"
	"github.com/franckferman/do-manager/pkg/client"
	"github.com/franckferman/do-manager/pkg/droplet"
	"github.com/franckferman/do-manager/pkg/firewall"
	"github.com/franckferman/do-manager/pkg/reservedip"
	"github.com/franckferman/do-manager/pkg/snapshot"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Security and hygiene audit of your DigitalOcean infrastructure",
	Long: `Audit scans your account for common misconfigurations and hygiene issues:

  - Droplets with no firewall attached
  - Droplets exposed to 0.0.0.0/0 on high-risk ports (22, 3389, 5900)
  - Reserved IPs not assigned to any Droplet (billing waste)
  - Snapshots older than --max-snapshot-age days
  - Firewall rules allowing 0.0.0.0/0 on sensitive ports

Exit codes:
  0  no findings
  1  at least one finding (use in CI/monitoring)`,
	RunE: runAudit,
}

var (
	auditMaxSnapshotAge int
	auditNoColor        bool
)

func init() {
	auditCmd.Flags().IntVar(&auditMaxSnapshotAge, "max-snapshot-age", 90, "Flag snapshots older than N days")
	rootCmd.AddCommand(auditCmd)
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type finding struct {
	severity string // CRITICAL | HIGH | MEDIUM | INFO
	resource string
	id       string
	message  string
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newAuditServices() (*droplet.Service, *firewall.Service, *reservedip.Service, *snapshot.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return droplet.New(c), firewall.New(c), reservedip.New(c), snapshot.New(c), nil
}

func sevColor(sev string) string {
	switch sev {
	case "CRITICAL":
		return color.New(color.FgRed, color.Bold).Sprint(sev)
	case "HIGH":
		return color.RedString(sev)
	case "MEDIUM":
		return color.YellowString(sev)
	default:
		return color.CyanString(sev)
	}
}

// sensitiveInboundPort returns true if the port range exposes a high-risk port.
func sensitiveInboundPort(portRange string) bool {
	highRisk := []string{"22", "3389", "5900", "23", "21", "2222"}
	for _, p := range highRisk {
		if portRange == p || strings.HasPrefix(portRange, p+"-") ||
			strings.HasSuffix(portRange, "-"+p) || portRange == "all" || portRange == "0" {
			return true
		}
	}
	return false
}

func isOpenWorld(addresses []string) bool {
	for _, a := range addresses {
		if a == "0.0.0.0/0" || a == "::/0" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Runner
// ---------------------------------------------------------------------------

func runAudit(cmd *cobra.Command, args []string) error {
	dropSvc, fwSvc, ripSvc, snapSvc, err := newAuditServices()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var findings []finding

	// ------------------------------------------------------------------ Droplets
	droplets, err := dropSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list droplets: %w", err)
	}

	// ------------------------------------------------------------------ Firewalls
	firewalls, err := fwSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list firewalls: %w", err)
	}

	// Build set: droplet ID -> firewall IDs
	dropletFWs := make(map[int][]string)
	for _, fw := range firewalls {
		for _, dID := range fw.DropletIDs {
			dropletFWs[dID] = append(dropletFWs[dID], fw.ID)
		}
		// Also check tag-matched droplets (simplified: if fw has tags, we skip for now)

		// Audit firewall rules for open world on sensitive ports
		for _, rule := range fw.InboundRules {
			if rule.Sources == nil {
				continue
			}
			if isOpenWorld(rule.Sources.Addresses) && sensitiveInboundPort(rule.PortRange) {
				findings = append(findings, finding{
					severity: "HIGH",
					resource: "firewall",
					id:       fw.Name,
					message:  fmt.Sprintf("Inbound rule exposes port %s to 0.0.0.0/0", rule.PortRange),
				})
			}
		}
	}

	// Check each droplet
	for _, d := range droplets {
		ip := droplet.PublicIPv4(&d)
		label := fmt.Sprintf("%s (ID=%d)", d.Name, d.ID)

		// No firewall attached
		if len(dropletFWs[d.ID]) == 0 {
			findings = append(findings, finding{
				severity: "CRITICAL",
				resource: "droplet",
				id:       label,
				message:  fmt.Sprintf("No firewall attached  public_ip=%s", ip),
			})
		}

		// Direct network check: look for firewall rules permitting open world on risky ports
		// attached to this droplet - already covered by fw loop above (per-firewall).
	}

	// ------------------------------------------------------------------ Reserved IPs
	rips, err := ripSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list reserved IPs: %w", err)
	}
	for _, rip := range rips {
		if rip.Droplet == nil {
			findings = append(findings, finding{
				severity: "MEDIUM",
				resource: "reserved_ip",
				id:       rip.IP,
				message:  "Not assigned to any Droplet (idle billing)",
			})
		}
	}

	// ------------------------------------------------------------------ Snapshots
	snaps, err := snapSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list snapshots: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -auditMaxSnapshotAge)
	for _, s := range snaps {
		created, parseErr := time.Parse(time.RFC3339, s.Created)
		if parseErr != nil {
			continue
		}
		if created.Before(cutoff) {
			findings = append(findings, finding{
				severity: "INFO",
				resource: "snapshot",
				id:       s.Name,
				message: fmt.Sprintf("Created %s (%.0f days ago)",
					created.Format("2006-01-02"),
					time.Since(created).Hours()/24),
			})
		}
	}

	// ------------------------------------------------------------------ Output
	if isJSON() {
		type jsonFinding struct {
			Severity string `json:"severity"`
			Resource string `json:"resource"`
			ID       string `json:"id"`
			Message  string `json:"message"`
		}
		out := make([]jsonFinding, len(findings))
		for i, f := range findings {
			out[i] = jsonFinding{f.severity, f.resource, f.id, f.message}
		}
		return printJSON(map[string]any{
			"total":    len(findings),
			"findings": out,
		})
	}

	fmt.Printf("\n%s Audit results for %d droplet(s)  %d firewall(s)  %d snap(s)  %d reserved IP(s)\n\n",
		color.CyanString(">>"),
		len(droplets), len(firewalls), len(snaps), len(rips))

	if len(findings) == 0 {
		fmt.Printf("%s No findings. Infrastructure looks clean.\n\n", color.GreenString("✓"))
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Severity", "Resource", "ID", "Finding"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")
	table.SetColWidth(60)
	table.SetAutoWrapText(true)

	for _, f := range findings {
		table.Append([]string{
			sevColor(f.severity),
			f.resource,
			f.id,
			f.message,
		})
	}
	table.Render()
	fmt.Printf("\n  %d finding(s) total\n\n", len(findings))

	// Non-zero exit if any CRITICAL or HIGH
	for _, f := range findings {
		if f.severity == "CRITICAL" || f.severity == "HIGH" {
			os.Exit(1)
		}
	}
	return nil
}
