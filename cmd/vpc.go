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
	"github.com/franckferman/do-manager/pkg/vpc"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var vpcCmd = &cobra.Command{
	Use:   "vpc",
	Short: "Manage Virtual Private Cloud networks",
	Long: `Manage DigitalOcean VPCs (Virtual Private Cloud).

A VPC is an isolated private network scoped to a single region.
Droplets inside a VPC communicate via private IPs without touching
the public internet.

Typical Red Team architecture:
  - Create a VPC in your target region
  - Deploy a redirector Droplet (public-facing, ports 80/443)
  - Deploy C2 Droplets with no public firewall rule for C2 port
  - Redirector -> C2 via VPC private IP only`,
}

var vpcListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all VPCs",
	RunE:    runVPCList,
}

var vpcGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show details of a VPC",
	Args:  cobra.ExactArgs(1),
	RunE:  runVPCGet,
}

var vpcCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new VPC",
	Long: `Create a new VPC in a region.

The IP range must be an RFC-1918 CIDR (10.x, 172.16-31.x, 192.168.x).
If omitted, DigitalOcean assigns a /20 automatically.

Examples:
  # Auto IP range
  do-manager vpc create --name c2-net --region fra1

  # Explicit /24
  do-manager vpc create --name lab-net --region nyc1 --ip-range 10.20.0.0/24`,
	RunE: runVPCCreate,
}

var vpcDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a VPC (must have no members)",
	Args:    cobra.ExactArgs(1),
	RunE:    runVPCDelete,
}

var vpcMembersCmd = &cobra.Command{
	Use:   "members <id>",
	Short: "List resources attached to a VPC",
	Args:  cobra.ExactArgs(1),
	RunE:  runVPCMembers,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var (
	vpcName        string
	vpcRegion      string
	vpcDescription string
	vpcIPRange     string
	vpcMemberType  string
)

func init() {
	vpcCreateCmd.Flags().StringVarP(&vpcName, "name", "n", "", "VPC name")
	vpcCreateCmd.Flags().StringVarP(&vpcRegion, "region", "r", "", "Region slug (e.g. fra1, nyc1)")
	vpcCreateCmd.Flags().StringVar(&vpcDescription, "description", "", "Optional description")
	vpcCreateCmd.Flags().StringVar(&vpcIPRange, "ip-range", "", "CIDR block (e.g. 10.20.0.0/24) - auto-assigned if omitted")
	vpcCreateCmd.MarkFlagRequired("name")   //nolint:errcheck
	vpcCreateCmd.MarkFlagRequired("region") //nolint:errcheck

	vpcMembersCmd.Flags().StringVar(&vpcMemberType, "type", "", "Filter by resource type: droplet, load_balancer, kubernetes (default: all)")

	vpcCmd.AddCommand(vpcListCmd, vpcGetCmd, vpcCreateCmd, vpcDeleteCmd, vpcMembersCmd)
	rootCmd.AddCommand(vpcCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newVPCService() (*vpc.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return vpc.New(c), nil
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runVPCList(cmd *cobra.Command, args []string) error {
	svc, err := newVPCService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	vpcs, err := svc.List(ctx)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(vpcs)
	}

	if len(vpcs) == 0 {
		fmt.Println(color.YellowString("No VPCs found."))
		return nil
	}

	fmt.Printf("\n%s  %d VPC(s)\n\n", color.CyanString(">>"), len(vpcs))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"ID", "Name", "Region", "IP Range", "Default", "Created"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, v := range vpcs {
		def := "-"
		if v.Default {
			def = color.GreenString("yes")
		}
		table.Append([]string{
			v.ID,
			v.Name,
			v.Region,
			v.IPRange,
			def,
			v.CreatedAt.Format("2006-01-02"),
		})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runVPCGet(cmd *cobra.Command, args []string) error {
	svc, err := newVPCService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	v, err := svc.Get(ctx, args[0])
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(v)
	}

	label := color.CyanString
	fmt.Printf("\n%s VPC %s\n\n", color.CyanString(">>"), color.WhiteString(v.Name))
	fmt.Printf("  %-14s %s\n", label("ID:"), v.ID)
	fmt.Printf("  %-14s %s\n", label("Name:"), v.Name)
	fmt.Printf("  %-14s %s\n", label("Region:"), v.Region)
	fmt.Printf("  %-14s %s\n", label("IP Range:"), v.IPRange)
	fmt.Printf("  %-14s %v\n", label("Default:"), v.Default)
	if v.Description != "" {
		fmt.Printf("  %-14s %s\n", label("Description:"), v.Description)
	}
	fmt.Printf("  %-14s %s\n", label("Created:"), v.CreatedAt.Format(time.RFC3339))
	fmt.Println()

	fmt.Printf("  %s\n  # Attach a new Droplet to this VPC:\n  do-manager droplet create --vpc-uuid %s ...\n\n",
		color.CyanString("Usage:"), v.ID)
	return nil
}

func runVPCCreate(cmd *cobra.Command, args []string) error {
	svc, err := newVPCService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	v, err := svc.Create(ctx, vpc.CreateOptions{
		Name:        vpcName,
		Region:      vpcRegion,
		Description: vpcDescription,
		IPRange:     vpcIPRange,
	})
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(v)
	}

	fmt.Printf("%s VPC created\n", color.GreenString("✓"))
	fmt.Printf("  %-14s %s\n", color.CyanString("ID:"), v.ID)
	fmt.Printf("  %-14s %s\n", color.CyanString("Name:"), v.Name)
	fmt.Printf("  %-14s %s\n", color.CyanString("Region:"), v.Region)
	fmt.Printf("  %-14s %s\n", color.CyanString("IP Range:"), v.IPRange)
	fmt.Printf("\n  %s\n  do-manager droplet create --vpc-uuid %s --name <name> --region %s ...\n\n",
		color.CyanString("Next - attach a Droplet:"), v.ID, v.Region)
	return nil
}

func runVPCDelete(cmd *cobra.Command, args []string) error {
	svc, err := newVPCService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.Delete(ctx, args[0]); err != nil {
		return err
	}
	fmt.Printf("%s VPC %s deleted.\n", color.GreenString("✓"), color.WhiteString(args[0]))
	return nil
}

func runVPCMembers(cmd *cobra.Command, args []string) error {
	svc, err := newVPCService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	members, err := svc.Members(ctx, args[0], vpcMemberType)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(members)
	}

	if len(members) == 0 {
		fmt.Println(color.YellowString("No members found."))
		return nil
	}

	fmt.Printf("\n%s  %d member(s) in VPC %s\n\n",
		color.CyanString(">>"), len(members), color.WhiteString(args[0]))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Name", "URN", "Joined"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, m := range members {
		// Extract resource type from URN: "do:droplet:12345678" -> "droplet"
		resourceType := m.URN
		parts := strings.SplitN(m.URN, ":", 3)
		if len(parts) == 3 {
			resourceType = color.CyanString(parts[1]) + ":" + parts[2]
		}
		table.Append([]string{
			m.Name,
			resourceType,
			m.CreatedAt.Format("2006-01-02"),
		})
	}
	table.Render()
	fmt.Println()
	return nil
}
