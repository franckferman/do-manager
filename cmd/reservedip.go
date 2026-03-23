package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/franckferman/do-manager/internal/config"
	"github.com/franckferman/do-manager/pkg/client"
	"github.com/franckferman/do-manager/pkg/reservedip"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var reservedIPCmd = &cobra.Command{
	Use:     "reserved-ip",
	Aliases: []string{"rip", "floating-ip"},
	Short:   "Manage Reserved (Floating) IPs",
}

var reservedIPListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all reserved IPs",
	RunE:    runReservedIPList,
}

var reservedIPGetCmd = &cobra.Command{
	Use:   "get <ip>",
	Short: "Show details of a reserved IP",
	Args:  cobra.ExactArgs(1),
	RunE:  runReservedIPGet,
}

var reservedIPReserveCmd = &cobra.Command{
	Use:   "reserve",
	Short: "Reserve a new IP address in a region",
	RunE:  runReservedIPReserve,
}

var reservedIPDeleteCmd = &cobra.Command{
	Use:     "delete <ip>",
	Aliases: []string{"rm", "release"},
	Short:   "Release a reserved IP back to the pool",
	Args:    cobra.ExactArgs(1),
	RunE:    runReservedIPDelete,
}

var reservedIPAssignCmd = &cobra.Command{
	Use:   "assign <ip>",
	Short: "Assign a reserved IP to a Droplet",
	Args:  cobra.ExactArgs(1),
	RunE:  runReservedIPAssign,
}

var reservedIPUnassignCmd = &cobra.Command{
	Use:   "unassign <ip>",
	Short: "Detach a reserved IP from its Droplet",
	Args:  cobra.ExactArgs(1),
	RunE:  runReservedIPUnassign,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var (
	ripRegion    string
	ripDropletID int
)

func init() {
	reservedIPReserveCmd.Flags().StringVarP(&ripRegion, "region", "r", "", "Region slug (e.g. fra1, nyc1)")
	reservedIPReserveCmd.Flags().IntVarP(&ripDropletID, "droplet", "d", 0, "Assign immediately to this Droplet ID")
	reservedIPReserveCmd.MarkFlagRequired("region") //nolint:errcheck

	reservedIPAssignCmd.Flags().IntVarP(&ripDropletID, "droplet", "d", 0, "Target Droplet ID")
	reservedIPAssignCmd.MarkFlagRequired("droplet") //nolint:errcheck

	reservedIPCmd.AddCommand(
		reservedIPListCmd,
		reservedIPGetCmd,
		reservedIPReserveCmd,
		reservedIPDeleteCmd,
		reservedIPAssignCmd,
		reservedIPUnassignCmd,
	)
	rootCmd.AddCommand(reservedIPCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newReservedIPService() (*reservedip.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return reservedip.New(c), nil
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runReservedIPList(cmd *cobra.Command, args []string) error {
	svc, err := newReservedIPService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ips, err := svc.List(ctx)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(ips)
	}

	if len(ips) == 0 {
		fmt.Println(color.YellowString("No reserved IPs found."))
		return nil
	}

	fmt.Printf("\n%s  %d reserved IP(s)\n\n", color.CyanString(">>"), len(ips))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"IP", "Region", "Droplet ID", "Droplet Name"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, rip := range ips {
		region := "-"
		if rip.Region != nil {
			region = rip.Region.Slug
		}
		dropletID := "-"
		dropletName := "-"
		if rip.Droplet != nil {
			dropletID = strconv.Itoa(rip.Droplet.ID)
			dropletName = rip.Droplet.Name
		}
		table.Append([]string{
			color.CyanString(rip.IP),
			region,
			dropletID,
			dropletName,
		})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runReservedIPGet(cmd *cobra.Command, args []string) error {
	svc, err := newReservedIPService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rip, err := svc.Get(ctx, args[0])
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(rip)
	}

	label := color.CyanString
	fmt.Printf("\n%s Reserved IP %s\n\n", color.CyanString(">>"), color.WhiteString(rip.IP))
	region := "-"
	if rip.Region != nil {
		region = fmt.Sprintf("%s (%s)", rip.Region.Name, rip.Region.Slug)
	}
	fmt.Printf("  %-16s %s\n", label("IP:"), rip.IP)
	fmt.Printf("  %-16s %s\n", label("Region:"), region)
	if rip.Droplet != nil {
		fmt.Printf("  %-16s %d  (%s)\n", label("Droplet:"), rip.Droplet.ID, rip.Droplet.Name)
	} else {
		fmt.Printf("  %-16s %s\n", label("Droplet:"), color.YellowString("unassigned"))
	}
	fmt.Println()
	return nil
}

func runReservedIPReserve(cmd *cobra.Command, args []string) error {
	svc, err := newReservedIPService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rip, err := svc.Reserve(ctx, ripRegion, ripDropletID)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(rip)
	}

	assigned := "unassigned"
	if rip.Droplet != nil {
		assigned = fmt.Sprintf("droplet %d", rip.Droplet.ID)
	}
	fmt.Printf("%s Reserved IP %s  region=%s  assigned=%s\n",
		color.GreenString("✓"),
		color.CyanString(rip.IP),
		rip.Region.Slug,
		assigned,
	)
	return nil
}

func runReservedIPDelete(cmd *cobra.Command, args []string) error {
	svc, err := newReservedIPService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.Delete(ctx, args[0]); err != nil {
		return err
	}
	fmt.Printf("%s Reserved IP %s released.\n", color.GreenString("✓"), color.CyanString(args[0]))
	return nil
}

func runReservedIPAssign(cmd *cobra.Command, args []string) error {
	svc, err := newReservedIPService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	action, err := svc.Assign(ctx, args[0], ripDropletID)
	if err != nil {
		return err
	}
	fmt.Printf("%s Assign action initiated  IP=%s  droplet=%d  action_id=%d\n",
		color.GreenString("✓"),
		color.CyanString(args[0]),
		ripDropletID,
		action.ID,
	)
	return nil
}

func runReservedIPUnassign(cmd *cobra.Command, args []string) error {
	svc, err := newReservedIPService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	action, err := svc.Unassign(ctx, args[0])
	if err != nil {
		return err
	}
	fmt.Printf("%s Unassign action initiated  IP=%s  action_id=%d\n",
		color.GreenString("✓"),
		color.CyanString(args[0]),
		action.ID,
	)
	return nil
}
