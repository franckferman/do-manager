package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/franckferman/do-manager/internal/config"
	"github.com/franckferman/do-manager/pkg/client"
	"github.com/franckferman/do-manager/pkg/firewall"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var firewallCmd = &cobra.Command{
	Use:     "firewall",
	Aliases: []string{"fw"},
	Short:   "Manage Cloud Firewalls",
}

var firewallListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all firewalls",
	RunE:    runFirewallList,
}

var firewallGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show details of a firewall",
	Args:  cobra.ExactArgs(1),
	RunE:  runFirewallGet,
}

var firewallCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new firewall",
	Long: `Create a new Cloud Firewall.

Rules are specified as "proto:ports:addresses" strings.

  proto    : tcp | udp | icmp
  ports    : single port, range (80-90), or 0 for icmp
  addresses: comma-separated CIDRs or IPs, or "any" for 0.0.0.0/0,::/0

Examples:
  --inbound  "tcp:443:any"
  --inbound  "tcp:22:203.0.113.1,198.51.100.0/24"
  --outbound "tcp:0-65535:any"
  --outbound "icmp:0:any"`,
	RunE: runFirewallCreate,
}

var firewallDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a firewall",
	Args:    cobra.ExactArgs(1),
	RunE:    runFirewallDelete,
}

var firewallAttachCmd = &cobra.Command{
	Use:   "attach <firewall-id>",
	Short: "Attach Droplets to a firewall",
	Args:  cobra.ExactArgs(1),
	RunE:  runFirewallAttach,
}

var firewallDetachCmd = &cobra.Command{
	Use:   "detach <firewall-id>",
	Short: "Detach Droplets from a firewall",
	Args:  cobra.ExactArgs(1),
	RunE:  runFirewallDetach,
}

var firewallPresetCmd = &cobra.Command{
	Use:   "preset <profile>",
	Short: "Apply a named firewall preset to Droplets",
	Long: `Apply an opinionated firewall preset for common Red Team / lab roles.

Available profiles:
  c2          Command & Control server
               - Inbound:  TCP 443, TCP 80, TCP 53 from any; TCP 22 from operator-ip
               - Outbound: all

  phishing    Phishing / gophish server
               - Inbound:  TCP 443, TCP 80, TCP 8080 from any; TCP 22 from operator-ip
               - Outbound: all

  redirector  Traffic redirector (socat / nginx)
               - Inbound:  TCP 443, TCP 80 from any; TCP 22 from operator-ip
               - Outbound: TCP 443, TCP 80 to any

  bastion     Jump host / bastion
               - Inbound:  TCP 22 from operator-ip only
               - Outbound: all

  lockdown    Block everything except TCP 22 from operator-ip
               - Inbound:  TCP 22 from operator-ip
               - Outbound: none

Examples:
  do-manager firewall preset c2 --name my-c2-fw --droplets 12345678 --operator-ip 203.0.113.1
  do-manager firewall preset phishing --name gophish-fw --droplets 12345678,98765432 --operator-ip 203.0.113.1`,
	Args: cobra.ExactArgs(1),
	RunE: runFirewallPreset,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var (
	fwName       string
	fwInbound    []string
	fwOutbound   []string
	fwDroplets   []int
	fwTags       []string
	fwPresetName string
	fwOperatorIP string
)

func init() {
	firewallCreateCmd.Flags().StringVarP(&fwName, "name", "n", "", "Firewall name")
	firewallCreateCmd.Flags().StringArrayVar(&fwInbound, "inbound", nil, "Inbound rule: proto:ports:addresses (repeatable)")
	firewallCreateCmd.Flags().StringArrayVar(&fwOutbound, "outbound", nil, "Outbound rule: proto:ports:addresses (repeatable)")
	firewallCreateCmd.Flags().IntSliceVar(&fwDroplets, "droplets", nil, "Droplet IDs to attach (comma-separated)")
	firewallCreateCmd.Flags().StringSliceVar(&fwTags, "tags", nil, "Tags to apply the firewall to")
	firewallCreateCmd.MarkFlagRequired("name") //nolint:errcheck

	firewallAttachCmd.Flags().IntSliceVar(&fwDroplets, "droplets", nil, "Droplet IDs to attach (comma-separated)")
	firewallAttachCmd.MarkFlagRequired("droplets") //nolint:errcheck

	firewallDetachCmd.Flags().IntSliceVar(&fwDroplets, "droplets", nil, "Droplet IDs to detach (comma-separated)")
	firewallDetachCmd.MarkFlagRequired("droplets") //nolint:errcheck

	firewallPresetCmd.Flags().StringVarP(&fwPresetName, "name", "n", "", "Firewall name (defaults to <profile>-fw)")
	firewallPresetCmd.Flags().StringVar(&fwOperatorIP, "operator-ip", "", "Your operator IP for SSH access (required for profiles that restrict port 22)")
	firewallPresetCmd.Flags().IntSliceVar(&fwDroplets, "droplets", nil, "Droplet IDs to attach the firewall to")
	firewallPresetCmd.Flags().StringSliceVar(&fwTags, "tags", nil, "Tags to apply the firewall to")

	firewallCmd.AddCommand(
		firewallListCmd,
		firewallGetCmd,
		firewallCreateCmd,
		firewallDeleteCmd,
		firewallAttachCmd,
		firewallDetachCmd,
		firewallPresetCmd,
	)
	rootCmd.AddCommand(firewallCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newFirewallService() (*firewall.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return firewall.New(c), nil
}

func parseRules(specs []string) ([]firewall.RuleSpec, error) {
	rules := make([]firewall.RuleSpec, 0, len(specs))
	for _, s := range specs {
		r, err := firewall.ParseRuleSpec(s)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runFirewallList(cmd *cobra.Command, args []string) error {
	svc, err := newFirewallService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fws, err := svc.List(ctx)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(fws)
	}

	if len(fws) == 0 {
		fmt.Println(color.YellowString("No firewalls found."))
		return nil
	}

	fmt.Printf("\n%s  %d firewall(s)\n\n", color.CyanString(">>"), len(fws))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"ID", "Name", "Status", "Droplets", "Tags", "Created"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, fw := range fws {
		ids := make([]string, len(fw.DropletIDs))
		for i, id := range fw.DropletIDs {
			ids[i] = strconv.Itoa(id)
		}
		dropletStr := "-"
		if len(ids) > 0 {
			dropletStr = strings.Join(ids, ",")
		}
		tagStr := "-"
		if len(fw.Tags) > 0 {
			tagStr = strings.Join(fw.Tags, ",")
		}
		statusStr := fw.Status
		switch fw.Status {
		case "succeeded":
			statusStr = color.GreenString(fw.Status)
		case "failed":
			statusStr = color.RedString(fw.Status)
		}
		table.Append([]string{
			fw.ID,
			fw.Name,
			statusStr,
			dropletStr,
			tagStr,
			fw.Created,
		})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runFirewallGet(cmd *cobra.Command, args []string) error {
	svc, err := newFirewallService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fw, err := svc.Get(ctx, args[0])
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(fw)
	}

	label := color.CyanString
	fmt.Printf("\n%s Firewall %s\n\n", color.CyanString(">>"), color.WhiteString(fw.Name))
	fmt.Printf("  %-14s %s\n", label("ID:"), fw.ID)
	fmt.Printf("  %-14s %s\n", label("Status:"), fw.Status)
	fmt.Printf("  %-14s %d attached\n", label("Droplets:"), len(fw.DropletIDs))
	fmt.Printf("  %-14s %s\n", label("Tags:"), strings.Join(fw.Tags, ", "))
	fmt.Printf("  %-14s %s\n", label("Created:"), fw.Created)

	if len(fw.InboundRules) > 0 {
		fmt.Printf("\n  %s\n", label("Inbound Rules:"))
		for _, r := range fw.InboundRules {
			addrs := "-"
			if r.Sources != nil {
				addrs = strings.Join(r.Sources.Addresses, ", ")
			}
			fmt.Printf("    %s  port=%-12s  from=%s\n",
				color.GreenString("IN "), r.PortRange, addrs)
		}
	}
	if len(fw.OutboundRules) > 0 {
		fmt.Printf("\n  %s\n", label("Outbound Rules:"))
		for _, r := range fw.OutboundRules {
			addrs := "-"
			if r.Destinations != nil {
				addrs = strings.Join(r.Destinations.Addresses, ", ")
			}
			fmt.Printf("    %s  port=%-12s  to=%s\n",
				color.YellowString("OUT"), r.PortRange, addrs)
		}
	}
	fmt.Println()
	return nil
}

func runFirewallCreate(cmd *cobra.Command, args []string) error {
	inRules, err := parseRules(fwInbound)
	if err != nil {
		return fmt.Errorf("invalid --inbound: %w", err)
	}
	outRules, err := parseRules(fwOutbound)
	if err != nil {
		return fmt.Errorf("invalid --outbound: %w", err)
	}

	svc, err := newFirewallService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fw, err := svc.Create(ctx, firewall.CreateOptions{
		Name:          fwName,
		InboundRules:  inRules,
		OutboundRules: outRules,
		DropletIDs:    fwDroplets,
		Tags:          fwTags,
	})
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(fw)
	}

	fmt.Printf("%s Firewall created  ID=%s  name=%s  inbound=%d  outbound=%d\n",
		color.GreenString("✓"),
		color.WhiteString(fw.ID),
		fw.Name,
		len(fw.InboundRules),
		len(fw.OutboundRules),
	)
	return nil
}

func runFirewallDelete(cmd *cobra.Command, args []string) error {
	svc, err := newFirewallService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.Delete(ctx, args[0]); err != nil {
		return err
	}
	fmt.Printf("%s Firewall %s deleted.\n", color.GreenString("✓"), color.WhiteString(args[0]))
	return nil
}

func runFirewallAttach(cmd *cobra.Command, args []string) error {
	svc, err := newFirewallService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.AttachDroplets(ctx, args[0], fwDroplets...); err != nil {
		return err
	}
	fmt.Printf("%s Droplets %v attached to firewall %s\n",
		color.GreenString("✓"), fwDroplets, color.WhiteString(args[0]))
	return nil
}

func runFirewallDetach(cmd *cobra.Command, args []string) error {
	svc, err := newFirewallService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.DetachDroplets(ctx, args[0], fwDroplets...); err != nil {
		return err
	}
	fmt.Printf("%s Droplets %v detached from firewall %s\n",
		color.GreenString("✓"), fwDroplets, color.WhiteString(args[0]))
	return nil
}

// ---------------------------------------------------------------------------
// Presets
// ---------------------------------------------------------------------------

// presetRules returns the inbound and outbound RuleSpecs for a named profile.
// operatorIP is used for restricting SSH; pass "" to skip the SSH rule.
func presetRules(profile, operatorIP string) (in []firewall.RuleSpec, out []firewall.RuleSpec, err error) {
	sshRule := "tcp:22:any"
	if operatorIP != "" {
		sshRule = "tcp:22:" + operatorIP
	}

	switch strings.ToLower(profile) {
	case "c2":
		in = mustParseRules([]string{
			"tcp:443:any",
			"tcp:80:any",
			"tcp:53:any",
			"udp:53:any",
			sshRule,
		})
		out = mustParseRules([]string{"tcp:0-65535:any", "udp:0-65535:any", "icmp:0:any"})

	case "phishing":
		in = mustParseRules([]string{
			"tcp:443:any",
			"tcp:80:any",
			"tcp:8080:any",
			sshRule,
		})
		out = mustParseRules([]string{"tcp:0-65535:any", "udp:0-65535:any", "icmp:0:any"})

	case "redirector":
		in = mustParseRules([]string{
			"tcp:443:any",
			"tcp:80:any",
			sshRule,
		})
		out = mustParseRules([]string{
			"tcp:443:any",
			"tcp:80:any",
		})

	case "bastion":
		if operatorIP == "" {
			return nil, nil, fmt.Errorf("bastion preset requires --operator-ip")
		}
		in = mustParseRules([]string{sshRule})
		out = mustParseRules([]string{"tcp:0-65535:any", "udp:0-65535:any", "icmp:0:any"})

	case "lockdown":
		if operatorIP == "" {
			return nil, nil, fmt.Errorf("lockdown preset requires --operator-ip")
		}
		in = mustParseRules([]string{sshRule})
		out = nil

	default:
		return nil, nil, fmt.Errorf("unknown preset %q: choose c2, phishing, redirector, bastion, lockdown", profile)
	}
	return in, out, nil
}

// mustParseRules panics if any rule string is malformed (only used with hard-coded strings above).
func mustParseRules(specs []string) []firewall.RuleSpec {
	rules := make([]firewall.RuleSpec, 0, len(specs))
	for _, s := range specs {
		r, err := firewall.ParseRuleSpec(s)
		if err != nil {
			panic("bad preset rule: " + s + ": " + err.Error())
		}
		rules = append(rules, r)
	}
	return rules
}

func runFirewallPreset(cmd *cobra.Command, args []string) error {
	profile := args[0]

	inRules, outRules, err := presetRules(profile, fwOperatorIP)
	if err != nil {
		return err
	}

	name := fwPresetName
	if name == "" {
		name = profile + "-fw"
	}

	svc, err := newFirewallService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fw, err := svc.Create(ctx, firewall.CreateOptions{
		Name:          name,
		InboundRules:  inRules,
		OutboundRules: outRules,
		DropletIDs:    fwDroplets,
		Tags:          fwTags,
	})
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(fw)
	}

	fmt.Printf("%s Firewall preset %s applied\n", color.GreenString("✓"), color.CyanString(profile))
	fmt.Printf("  %-14s %s\n", color.CyanString("ID:"), fw.ID)
	fmt.Printf("  %-14s %s\n", color.CyanString("Name:"), fw.Name)
	fmt.Printf("  %-14s %d inbound  %d outbound\n",
		color.CyanString("Rules:"), len(fw.InboundRules), len(fw.OutboundRules))
	if len(fwDroplets) > 0 {
		fmt.Printf("  %-14s %v\n", color.CyanString("Droplets:"), fwDroplets)
	}
	return nil
}
