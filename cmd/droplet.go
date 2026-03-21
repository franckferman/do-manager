package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/fatih/color"
	"github.com/franckferman/do-manager/internal/config"
	"github.com/franckferman/do-manager/pkg/client"
	"github.com/franckferman/do-manager/pkg/droplet"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// Timeout / polling constants used across droplet commands.
const (
	apiTimeout   = 15 * time.Second
	createSingle = 60 * time.Second
	createBatch  = 120 * time.Second
	waitTimeout  = 5 * time.Minute
	pollInterval = 5 * time.Second
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var dropletCmd = &cobra.Command{
	Use:     "droplet",
	Aliases: []string{"d", "droplets"},
	Short:   "Manage DigitalOcean Droplets",
}

var dropletListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all Droplets",
	RunE:    runDropletList,
}

var dropletGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show details of a Droplet",
	Args:  cobra.ExactArgs(1),
	RunE:  runDropletGet,
}

var dropletCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Provision a new Droplet",
	RunE:  runDropletCreate,
}

var dropletDeleteCmd = &cobra.Command{
	Use:     "delete <id> [<id>...]",
	Aliases: []string{"rm", "destroy"},
	Short:   "Delete one or more Droplets by ID, or all Droplets with a tag",
	Args:    cobra.ArbitraryArgs,
	RunE:    runDropletDelete,
}

var dropletPowerCmd = &cobra.Command{
	Use:   "power",
	Short: "Power on/off or reboot a Droplet",
}

var dropletPowerOnCmd = &cobra.Command{
	Use:   "on <id>",
	Short: "Power on a Droplet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return dropletAction(args[0], "power-on", func(svc *droplet.Service, id int) error {
			return svc.PowerOn(context.Background(), id)
		})
	},
}

var dropletPowerOffCmd = &cobra.Command{
	Use:   "off <id>",
	Short: "Power off a Droplet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return dropletAction(args[0], "power-off", func(svc *droplet.Service, id int) error {
			return svc.PowerOff(context.Background(), id)
		})
	},
}

var dropletRebootCmd = &cobra.Command{
	Use:   "reboot <id>",
	Short: "Reboot a Droplet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return dropletAction(args[0], "reboot", func(svc *droplet.Service, id int) error {
			return svc.Reboot(context.Background(), id)
		})
	},
}

// ---------------------------------------------------------------------------
// Create flags
// ---------------------------------------------------------------------------

var (
	createName     string
	createRegion   string
	createSize     string
	createImage    string
	createSSHKeys  []int
	createTags     []string
	createUserData string
	createIPv6     bool
	createBackups  bool
	createWait     bool
	createCount    int
	deleteForce    bool
	deleteTag      string
)

func init() {
	// create flags
	dropletCreateCmd.Flags().StringVarP(&createName, "name", "n", "", "Droplet name (required)")
	dropletCreateCmd.Flags().StringVarP(&createRegion, "region", "r", "nyc1", "Region slug (e.g. nyc1, ams3, fra1)")
	dropletCreateCmd.Flags().StringVarP(&createSize, "size", "s", "s-1vcpu-1gb", "Size slug (e.g. s-1vcpu-1gb, s-2vcpu-4gb)")
	dropletCreateCmd.Flags().StringVarP(&createImage, "image", "i", "ubuntu-22-04-x64", "Image slug (e.g. ubuntu-22-04-x64)")
	dropletCreateCmd.Flags().IntSliceVar(&createSSHKeys, "ssh-keys", nil, "SSH key IDs to embed (comma-separated)")
	dropletCreateCmd.Flags().StringSliceVar(&createTags, "tags", nil, "Tags to apply (comma-separated)")
	dropletCreateCmd.Flags().StringVar(&createUserData, "user-data", "", "User-data / cloud-init script content")
	dropletCreateCmd.Flags().BoolVar(&createIPv6, "ipv6", false, "Enable IPv6")
	dropletCreateCmd.Flags().BoolVar(&createBackups, "backups", false, "Enable automatic backups")
	dropletCreateCmd.Flags().BoolVarP(&createWait, "wait", "w", false, "Wait until the Droplet is active and print its IP")
	dropletCreateCmd.Flags().IntVarP(&createCount, "count", "c", 1, "Number of Droplets to provision in parallel (names become name-01, name-02, ...)")
	dropletCreateCmd.MarkFlagRequired("name") //nolint:errcheck

	// delete flags
	dropletDeleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	dropletDeleteCmd.Flags().StringVar(&deleteTag, "tag", "", "Delete all Droplets carrying this tag")

	// assemble tree
	dropletPowerCmd.AddCommand(dropletPowerOnCmd, dropletPowerOffCmd, dropletRebootCmd)
	dropletCmd.AddCommand(
		dropletListCmd,
		dropletGetCmd,
		dropletCreateCmd,
		dropletDeleteCmd,
		dropletPowerCmd,
	)
	rootCmd.AddCommand(dropletCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newDropletService() (*droplet.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return droplet.New(c), nil
}

func statusColor(s string) string {
	switch strings.ToLower(s) {
	case "active":
		return color.GreenString(s)
	case "off":
		return color.RedString(s)
	case "new":
		return color.YellowString(s)
	case "archive":
		return color.HiBlackString(s)
	default:
		return s
	}
}

func printDropletsTable(droplets []godo.Droplet) {
	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"ID", "Name", "Status", "Region", "Size", "IPv4", "IPv6", "Tags"}
	table.SetHeader(headers)

	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)

	table.SetBorder(false)
	table.SetColumnSeparator(" | ")
	table.SetCenterSeparator("-")
	table.SetRowSeparator("-")
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, d := range droplets {
		ipv4 := droplet.PublicIPv4(&d)
		ipv6 := droplet.PublicIPv6(&d)
		if ipv6 == "" {
			ipv6 = "-"
		}
		table.Append([]string{
			strconv.Itoa(d.ID),
			d.Name,
			d.Status,
			d.Region.Slug,
			d.SizeSlug,
			ipv4,
			ipv6,
			strings.Join(d.Tags, ", "),
		})
	}
	table.Render()
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runDropletList(cmd *cobra.Command, args []string) error {
	svc, err := newDropletService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	droplets, err := svc.List(ctx)
	if err != nil {
		return err
	}
	if len(droplets) == 0 {
		fmt.Println(color.YellowString("No Droplets found."))
		return nil
	}

	fmt.Printf("\n%s  %d Droplet(s)\n\n", color.CyanString(">>"), len(droplets))
	printDropletsTable(droplets)
	fmt.Println()
	return nil
}

func runDropletGet(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid Droplet ID %q", args[0])
	}

	svc, err := newDropletService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	d, err := svc.Get(ctx, id)
	if err != nil {
		return err
	}

	ipv4 := droplet.PublicIPv4(d)
	ipv6 := droplet.PublicIPv6(d)

	label := color.CyanString
	fmt.Printf("\n%s Droplet %s\n\n", color.CyanString(">>"), color.WhiteString(d.Name))
	fmt.Printf("  %-14s %s\n", label("ID:"), strconv.Itoa(d.ID))
	fmt.Printf("  %-14s %s\n", label("Status:"), statusColor(d.Status))
	fmt.Printf("  %-14s %s (%s)\n", label("Region:"), d.Region.Name, d.Region.Slug)
	fmt.Printf("  %-14s %s\n", label("Size:"), d.SizeSlug)
	fmt.Printf("  %-14s %d vCPU  /  %d MB RAM  /  %d GB disk\n", label("Specs:"), d.Vcpus, d.Memory, d.Disk)
	fmt.Printf("  %-14s %s\n", label("IPv4:"), ipv4)
	if ipv6 != "" {
		fmt.Printf("  %-14s %s\n", label("IPv6:"), ipv6)
	}
	if len(d.Tags) > 0 {
		fmt.Printf("  %-14s %s\n", label("Tags:"), strings.Join(d.Tags, ", "))
	}
	fmt.Printf("  %-14s %s\n", label("Created:"), d.Created)
	fmt.Println()
	return nil
}

func runDropletCreate(cmd *cobra.Command, args []string) error {
	if createCount < 1 {
		return fmt.Errorf("--count must be >= 1")
	}

	svc, err := newDropletService()
	if err != nil {
		return err
	}

	opts := droplet.CreateOptions{
		Name:     createName,
		Region:   createRegion,
		Size:     createSize,
		Image:    createImage,
		SSHKeys:  createSSHKeys,
		Tags:     createTags,
		UserData: createUserData,
		IPv6:     createIPv6,
		Backups:  createBackups,
	}

	if createCount == 1 {
		// Single droplet path (unchanged behaviour)
		ctx, cancel := context.WithTimeout(context.Background(), createSingle)
		defer cancel()

		fmt.Printf("%s Creating %s  [region=%s  size=%s  image=%s]\n",
			color.CyanString(">>"), color.WhiteString(createName),
			createRegion, createSize, createImage,
		)

		d, err := svc.Create(ctx, opts)
		if err != nil {
			return err
		}
		fmt.Printf("%s Droplet created  ID=%s  status=%s\n",
			color.GreenString("✓"), color.WhiteString(strconv.Itoa(d.ID)), statusColor(d.Status),
		)
		if createWait {
			fmt.Printf("%s Waiting for active state", color.YellowString("~"))
			if err := waitForActive(svc, d.ID); err != nil {
				return err
			}
			refreshCtx, refreshCancel := context.WithTimeout(context.Background(), apiTimeout)
			defer refreshCancel()
			d, err = svc.Get(refreshCtx, d.ID)
			if err != nil {
				return err
			}
			fmt.Printf("\n%s Active  IPv4=%s\n",
				color.GreenString("✓"), color.WhiteString(droplet.PublicIPv4(d)),
			)
		}
		return nil
	}

	// Batch path
	fmt.Printf("%s Creating %d Droplets in parallel  [base=%s  region=%s  size=%s  image=%s]\n",
		color.CyanString(">>"), createCount, color.WhiteString(createName),
		createRegion, createSize, createImage,
	)

	ctx, cancel := context.WithTimeout(context.Background(), createBatch)
	defer cancel()

	results := svc.CreateBatch(ctx, opts, createCount)

	var failed int
	var ids []int
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("  %s %-20s %s\n", color.RedString("✗"), r.Name, r.Err)
			failed++
		} else {
			fmt.Printf("  %s %-20s ID=%-12d status=%s\n",
				color.GreenString("✓"), r.Name,
				r.Droplet.ID, statusColor(r.Droplet.Status),
			)
			ids = append(ids, r.Droplet.ID)
		}
	}

	if failed > 0 {
		fmt.Printf("\n%s %d/%d Droplets failed.\n", color.RedString("!"), failed, createCount)
	}

	if createWait && len(ids) > 0 {
		fmt.Printf("\n%s Waiting for %d Droplet(s) to become active", color.YellowString("~"), len(ids))
		waitResults := waitForActiveMany(svc, ids)
		fmt.Println()
		for id, ip := range waitResults {
			if ip == "" {
				fmt.Printf("  %s ID=%-12d timed out\n", color.RedString("✗"), id)
			} else {
				fmt.Printf("  %s ID=%-12d IPv4=%s\n", color.GreenString("✓"), id, color.WhiteString(ip))
			}
		}
	}

	return nil
}

func runDropletDelete(cmd *cobra.Command, args []string) error {
	if deleteTag == "" && len(args) == 0 {
		return fmt.Errorf("provide at least one Droplet ID or use --tag <tag>")
	}

	svc, err := newDropletService()
	if err != nil {
		return err
	}

	// Tag-based deletion
	if deleteTag != "" {
		if !deleteForce {
			fmt.Printf("%s Delete ALL Droplets with tag %s? This is irreversible. [y/N]: ",
				color.YellowString("!"), color.WhiteString(deleteTag),
			)
			var reply string
			fmt.Scan(&reply)
			if strings.ToLower(strings.TrimSpace(reply)) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := svc.DeleteByTag(ctx, deleteTag); err != nil {
			return err
		}
		fmt.Printf("%s All Droplets with tag %s deleted.\n",
			color.GreenString("✓"), color.WhiteString(deleteTag),
		)
		return nil
	}

	// ID-based deletion (one or many)
	ids := make([]int, 0, len(args))
	for _, a := range args {
		id, err := strconv.Atoi(a)
		if err != nil {
			return fmt.Errorf("invalid Droplet ID %q", a)
		}
		ids = append(ids, id)
	}

	if !deleteForce {
		fmt.Printf("%s Delete Droplet(s) %s? This is irreversible. [y/N]: ",
			color.YellowString("!"), color.WhiteString(strings.Join(args, ", ")),
		)
		var reply string
		fmt.Scan(&reply)
		if strings.ToLower(strings.TrimSpace(reply)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(ids) == 1 {
		if err := svc.Delete(ctx, ids[0]); err != nil {
			return err
		}
		fmt.Printf("%s Droplet %s deleted.\n", color.GreenString("✓"), color.WhiteString(args[0]))
		return nil
	}

	// Parallel multi-delete
	errs := svc.DeleteMany(ctx, ids)
	for i, err := range errs {
		if err != nil {
			fmt.Printf("  %s ID=%-12d %s\n", color.RedString("✗"), ids[i], err)
		} else {
			fmt.Printf("  %s ID=%-12d deleted\n", color.GreenString("✓"), ids[i])
		}
	}
	return nil
}

func dropletAction(idStr, action string, fn func(*droplet.Service, int) error) error {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return fmt.Errorf("invalid Droplet ID %q", idStr)
	}
	svc, err := newDropletService()
	if err != nil {
		return err
	}
	if err := fn(svc, id); err != nil {
		return err
	}
	fmt.Printf("%s Droplet %s: %s initiated.\n",
		color.GreenString("✓"),
		color.WhiteString(strconv.Itoa(id)),
		action,
	)
	return nil
}

func waitForActive(svc *droplet.Service, id int) error {
	timeout := time.After(waitTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout: Droplet %d did not become active within %s", id, waitTimeout)
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
			d, err := svc.Get(ctx, id)
			cancel()
			if err != nil {
				return err
			}
			if d.Status == "active" {
				return nil
			}
			fmt.Print(".")
		}
	}
}

// waitForActiveMany polls all given IDs in parallel until active or timeout.
// Returns a map[id]ipv4 (empty string on timeout).
func waitForActiveMany(svc *droplet.Service, ids []int) map[int]string {
	type result struct {
		id  int
		ip  string
	}
	ch := make(chan result, len(ids))

	for _, id := range ids {
		go func(dropletID int) {
			defer func() {
				if r := recover(); r != nil {
					ch <- result{id: dropletID, ip: ""}
				}
			}()
			timeout := time.After(waitTimeout)
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-timeout:
					ch <- result{id: dropletID, ip: ""}
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
					d, err := svc.Get(ctx, dropletID)
					cancel()
					if err == nil && d.Status == "active" {
						ch <- result{id: dropletID, ip: droplet.PublicIPv4(d)}
						fmt.Print(".")
						return
					}
					fmt.Print(".")
				}
			}
		}(id)
	}

	out := make(map[int]string, len(ids))
	for range ids {
		r := <-ch
		out[r.id] = r.ip
	}
	return out
}
