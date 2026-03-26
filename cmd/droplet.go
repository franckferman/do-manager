package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/digitalocean/godo"
	"github.com/fatih/color"
	"github.com/franckferman/do-manager/internal/config"
	"github.com/franckferman/do-manager/pkg/client"
	"github.com/franckferman/do-manager/pkg/droplet"
	"github.com/franckferman/do-manager/pkg/tmpl"
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

var dropletIPsCmd = &cobra.Command{
	Use:   "ips",
	Short: "Print public IPv4 addresses, one per line",
	Long: `Print the public IPv4 of each Droplet, one per line.

Useful for piping directly into tools like nmap, masscan, or Python scripts:

  do-manager droplet ips --tag lab | xargs nmap -sV -p 443
  do-manager droplet ips --tag c2  | python3 setup_c2.py`,
	RunE: runDropletIPs,
}

var dropletSSHCmd = &cobra.Command{
	Use:   "ssh <id-or-name>",
	Short: "Open an SSH session to a Droplet",
	Args:  cobra.ExactArgs(1),
	RunE:  runDropletSSH,
}

var dropletExecCmd = &cobra.Command{
	Use:   "exec [--tag <tag>] -- <command>",
	Short: "Run a shell command on all Droplets matching a tag (parallel)",
	Long: `Execute a command via SSH on all Droplets matching a tag, in parallel.

Output is labeled per Droplet and buffered so lines don't interleave.

Examples:
  do-manager droplet exec --tag c2 -- "systemctl status gophish"
  do-manager droplet exec --tag lab -- "uname -r"
  do-manager droplet exec --tag web --user ubuntu -- "df -h"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDropletExec,
}

var dropletRebuildCmd = &cobra.Command{
	Use:   "rebuild <id>",
	Short: "Wipe and re-image a Droplet from a new image/snapshot",
	Long: `Rebuild (re-image) a Droplet from an image slug or snapshot slug.

The Droplet keeps its ID, reserved IPs, firewall assignments, and tags.
Only the disk is wiped and re-installed. Faster than delete + re-create.

Use cases:
  - A node is compromised or burned: rebuild with a clean image
  - Deploy a different role on existing infrastructure
  - Burn-and-refresh without changing the IP (reserved IP stays assigned)

Examples:
  do-manager droplet rebuild 12345678 --image ubuntu-22-04-x64
  do-manager droplet rebuild 12345678 --image gophish-snapshot --wait`,
	Args: cobra.ExactArgs(1),
	RunE: runDropletRebuild,
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
	createUserData     string
	createUserDataFile string
	createVPCUUID      string
	createIPv6         bool
	rebuildImage       string
	rebuildWait        bool
	createBackups  bool
	createWait     bool
	createCount    int
	deleteForce    bool
	deleteTag      string
	ipsTag         string
	execTag        string
	execUser       string
	sshUser        string
	sshPort        int
	createTemplate    string
	createTemplateVar []string
	createPassword    string
)

func init() {
	// create flags
	dropletCreateCmd.Flags().StringVarP(&createName, "name", "n", "", "Droplet name (required)")
	dropletCreateCmd.Flags().StringVarP(&createRegion, "region", "r", "nyc1", "Region slug (e.g. nyc1, ams3, fra1)")
	dropletCreateCmd.Flags().StringVarP(&createSize, "size", "s", "s-1vcpu-1gb", "Size slug (e.g. s-1vcpu-1gb, s-2vcpu-4gb)")
	dropletCreateCmd.Flags().StringVarP(&createImage, "image", "i", "ubuntu-22-04-x64", "Image slug (e.g. ubuntu-22-04-x64)")
	dropletCreateCmd.Flags().IntSliceVar(&createSSHKeys, "ssh-keys", nil, "SSH key IDs to embed (comma-separated)")
	dropletCreateCmd.Flags().StringSliceVar(&createTags, "tags", nil, "Tags to apply (comma-separated)")
	dropletCreateCmd.Flags().StringVar(&createUserData, "user-data", "", "Cloud-init script content (inline)")
	dropletCreateCmd.Flags().StringVar(&createUserDataFile, "user-data-file", "", "Path to a cloud-init / bash script file")
	dropletCreateCmd.Flags().StringVar(&createVPCUUID, "vpc-uuid", "", "Place the Droplet inside this VPC (use 'do-manager vpc list' to find IDs)")
	dropletCreateCmd.Flags().StringVar(&createTemplate, "template", "", "Built-in cloud-init template (see: do-manager template list)")
	dropletCreateCmd.Flags().StringArrayVar(&createTemplateVar, "template-var", nil, "Template variable: KEY=VALUE (repeatable, used with --template or --user-data-file)")
	dropletCreateCmd.Flags().BoolVar(&createIPv6, "ipv6", false, "Enable IPv6")
	dropletCreateCmd.Flags().BoolVar(&createBackups, "backups", false, "Enable automatic backups")
	dropletCreateCmd.Flags().BoolVarP(&createWait, "wait", "w", false, "Wait until the Droplet is active and print its IP")
	dropletCreateCmd.Flags().IntVarP(&createCount, "count", "c", 1, "Number of Droplets to provision in parallel (names become name-01, name-02, ...)")
	dropletCreateCmd.Flags().StringVar(&createPassword, "password", "", "Set root password via cloud-init (injected into any --template or --user-data-file)")
	dropletCreateCmd.MarkFlagRequired("name") //nolint:errcheck

	// delete flags
	dropletDeleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	dropletDeleteCmd.Flags().StringVar(&deleteTag, "tag", "", "Delete all Droplets carrying this tag")

	// ips flags
	dropletIPsCmd.Flags().StringVar(&ipsTag, "tag", "", "Filter by tag")

	// ssh flags
	dropletSSHCmd.Flags().StringVarP(&sshUser, "user", "u", "root", "SSH username")
	dropletSSHCmd.Flags().IntVarP(&sshPort, "port", "p", 22, "SSH port")

	// exec flags
	dropletExecCmd.Flags().StringVar(&execTag, "tag", "", "Run on all Droplets with this tag")
	dropletExecCmd.Flags().StringVarP(&execUser, "user", "u", "root", "SSH username")
	dropletExecCmd.MarkFlagRequired("tag") //nolint:errcheck

	// rebuild flags
	dropletRebuildCmd.Flags().StringVarP(&rebuildImage, "image", "i", "", "Image slug or snapshot slug to rebuild from (required)")
	dropletRebuildCmd.Flags().BoolVarP(&rebuildWait, "wait", "w", false, "Wait until rebuild is complete before exiting")
	dropletRebuildCmd.MarkFlagRequired("image") //nolint:errcheck

	// assemble tree
	dropletPowerCmd.AddCommand(dropletPowerOnCmd, dropletPowerOffCmd, dropletRebootCmd)
	dropletCmd.AddCommand(
		dropletListCmd,
		dropletGetCmd,
		dropletCreateCmd,
		dropletDeleteCmd,
		dropletPowerCmd,
		dropletIPsCmd,
		dropletSSHCmd,
		dropletExecCmd,
		dropletRebuildCmd,
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

// resolveDropletID returns the numeric ID from either a raw int string
// or a Droplet name. On name lookup it fetches the full list and returns
// the first match.
func resolveDropletID(idOrName string) (int, error) {
	if id, err := strconv.Atoi(idOrName); err == nil {
		return id, nil
	}
	// Name lookup
	svc, err := newDropletService()
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	droplets, err := svc.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, d := range droplets {
		if d.Name == idOrName {
			return d.ID, nil
		}
	}
	return 0, fmt.Errorf("no Droplet found with name or ID %q", idOrName)
}

// resolveUserData returns the effective user-data string.
// inline takes precedence; if empty, path is read from disk.
// Kept for backward compatibility; delegates to resolveCloudInit with no template.
func resolveUserData(inline, path string) (string, error) {
	return resolveCloudInit(inline, path, "", nil)
}

// resolveCloudInit returns the effective cloud-init script.
// Priority: file > template name > inline string.
// If vars are provided they are applied as Go text/template substitutions to
// the file content or the named built-in template.
func resolveCloudInit(inline, path, templateName string, vars map[string]string) (string, error) {
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read user-data-file %q: %w", path, err)
		}
		if len(vars) == 0 {
			return string(raw), nil
		}
		t, err := template.New("file").Option("missingkey=zero").Parse(string(raw))
		if err != nil {
			return string(raw), nil
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, vars); err != nil {
			return string(raw), nil
		}
		return buf.String(), nil
	}
	if templateName != "" {
		return tmpl.Render(templateName, vars)
	}
	return inline, nil
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

	if isJSON() {
		return printJSON(droplets)
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

	if isJSON() {
		return printJSON(d)
	}

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

// injectPassword returns user-data with root password auth configured.
//
// If no existing script: emits a #cloud-config (native cloud-init, handles
// sshd_config.d overrides automatically via ssh_pwauth).
// If existing script is provided: wraps both into a multipart MIME doc so
// cloud-init processes the cloud-config AND the bash script.
func injectPassword(script, password string) string {
	cloudConfig := fmt.Sprintf(
		"#cloud-config\nchpasswd:\n  list: |\n    root:%s\n  expire: false\nssh_pwauth: true\n",
		password,
	)
	if script == "" {
		return cloudConfig
	}
	boundary := "DO_MANAGER_MIME_BOUNDARY"
	return "Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\nMIME-Version: 1.0\n\n" +
		"--" + boundary + "\n" +
		"Content-Type: text/cloud-config; charset=\"us-ascii\"\n\n" +
		cloudConfig + "\n" +
		"--" + boundary + "\n" +
		"Content-Type: text/x-shellscript; charset=\"us-ascii\"\n\n" +
		script + "\n" +
		"--" + boundary + "--\n"
}

func runDropletCreate(cmd *cobra.Command, args []string) error {
	if createCount < 1 {
		return fmt.Errorf("--count must be >= 1")
	}

	ud, err := resolveCloudInit(createUserData, createUserDataFile, createTemplate, parseTmplVars(createTemplateVar))
	if err != nil {
		return err
	}
	if createPassword != "" {
		ud = injectPassword(ud, createPassword)
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
		UserData: ud,
		IPv6:     createIPv6,
		Backups:  createBackups,
		VPCUUID:  createVPCUUID,
	}

	if createCount == 1 {
		// Single droplet path
		ctx, cancel := context.WithTimeout(context.Background(), createSingle)
		defer cancel()

		if !isJSON() {
			fmt.Printf("%s Creating %s  [region=%s  size=%s  image=%s]\n",
				color.CyanString(">>"), color.WhiteString(createName),
				createRegion, createSize, createImage,
			)
		}

		d, err := svc.Create(ctx, opts)
		if err != nil {
			return err
		}

		if createWait {
			if !isJSON() {
				fmt.Printf("%s Waiting for active state", color.YellowString("~"))
			} else {
				fmt.Fprintf(os.Stderr, "Waiting for active state")
			}
			if err := waitForActive(svc, d.ID); err != nil {
				return err
			}
			refreshCtx, refreshCancel := context.WithTimeout(context.Background(), apiTimeout)
			defer refreshCancel()
			d, err = svc.Get(refreshCtx, d.ID)
			if err != nil {
				return err
			}
			if !isJSON() {
				fmt.Printf("\n%s Active  IPv4=%s\n",
					color.GreenString("✓"), color.WhiteString(droplet.PublicIPv4(d)),
				)
			} else {
				fmt.Fprintln(os.Stderr)
			}
		}

		if isJSON() {
			return printJSON(d)
		}
		fmt.Printf("%s Droplet created  ID=%s  status=%s\n",
			color.GreenString("✓"), color.WhiteString(strconv.Itoa(d.ID)), statusColor(d.Status),
		)
		if createPassword != "" {
			fmt.Printf("%s Password         %s\n",
				color.GreenString("✓"), color.YellowString(createPassword),
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
			if isJSON() {
				fmt.Fprint(os.Stderr, ".")
			} else {
				fmt.Print(".")
			}
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

// ---------------------------------------------------------------------------
// Runners: ips, ssh, exec
// ---------------------------------------------------------------------------

func runDropletIPs(cmd *cobra.Command, args []string) error {
	svc, err := newDropletService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	all, err := svc.List(ctx)
	if err != nil {
		return err
	}

	var filtered []godo.Droplet
	for _, d := range all {
		if ipsTag == "" {
			filtered = append(filtered, d)
			continue
		}
		for _, t := range d.Tags {
			if t == ipsTag {
				filtered = append(filtered, d)
				break
			}
		}
	}

	if len(filtered) == 0 {
		fmt.Fprintln(os.Stderr, color.YellowString("No Droplets found."))
		return nil
	}

	for _, d := range filtered {
		ip := droplet.PublicIPv4(&d)
		if ip != "" {
			fmt.Println(ip)
		}
	}
	return nil
}

func runDropletRebuild(cmd *cobra.Command, args []string) error {
	id, err := resolveDropletID(args[0])
	if err != nil {
		return err
	}

	svc, err := newDropletService()
	if err != nil {
		return err
	}

	getCtx, getCancel := context.WithTimeout(context.Background(), apiTimeout)
	d, err := svc.Get(getCtx, id)
	getCancel()
	if err != nil {
		return err
	}

	fmt.Printf("%s Rebuilding %s (ID=%d)  ip=%s  image=%s\n",
		color.YellowString("~"), color.WhiteString(d.Name), id,
		color.CyanString(droplet.PublicIPv4(d)), color.WhiteString(rebuildImage))

	rebuildCtx, rebuildCancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer rebuildCancel()

	action, err := svc.Rebuild(rebuildCtx, id, rebuildImage)
	if err != nil {
		return err
	}

	if !rebuildWait {
		fmt.Printf("%s Rebuild triggered  action_id=%d\n"+
			"  Takes ~5-10 min. Poll with:  do-manager droplet get %d\n",
			color.GreenString("✓"), action.ID, id)
		return nil
	}

	fmt.Printf("%s Waiting for rebuild", color.YellowString("~"))
	if err := svc.WaitRebuild(rebuildCtx, id, action.ID); err != nil {
		fmt.Println()
		return err
	}
	fmt.Println()
	fmt.Printf("%s Droplet %s rebuilt successfully  ip=%s\n",
		color.GreenString("✓"), color.WhiteString(d.Name), color.CyanString(droplet.PublicIPv4(d)))
	return nil
}

func runDropletSSH(cmd *cobra.Command, args []string) error {
	svc, err := newDropletService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Resolve by ID or name
	var ip string
	if id, err := strconv.Atoi(args[0]); err == nil {
		d, err := svc.Get(ctx, id)
		if err != nil {
			return err
		}
		ip = droplet.PublicIPv4(d)
	} else {
		// lookup by name
		all, err := svc.List(ctx)
		if err != nil {
			return err
		}
		for _, d := range all {
			if d.Name == args[0] {
				ip = droplet.PublicIPv4(&d)
				break
			}
		}
		if ip == "" {
			return fmt.Errorf("no Droplet found with name %q", args[0])
		}
	}

	if ip == "" {
		return fmt.Errorf("Droplet has no public IPv4 yet")
	}

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	target := fmt.Sprintf("%s@%s", sshUser, ip)
	portStr := strconv.Itoa(sshPort)
	fmt.Printf("%s ssh %s (port %s)\n", color.CyanString(">>"), color.WhiteString(target), portStr)

	c := exec.Command(sshBin,
		"-p", portStr,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		target,
	)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func runDropletExec(cmd *cobra.Command, args []string) error {
	remoteCmd := strings.Join(args, " ")

	svc, err := newDropletService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	all, err := svc.List(ctx)
	if err != nil {
		return err
	}

	var targets []godo.Droplet
	for _, d := range all {
		for _, t := range d.Tags {
			if t == execTag {
				targets = append(targets, d)
				break
			}
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("no Droplets found with tag %q", execTag)
	}

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	fmt.Printf("%s Executing on %d Droplet(s) [tag=%s]: %s\n\n",
		color.CyanString(">>"), len(targets), execTag, color.WhiteString(remoteCmd))

	type execResult struct {
		name   string
		ip     string
		output string
		err    error
	}

	results := make([]execResult, len(targets))
	var wg sync.WaitGroup

	for i, d := range targets {
		wg.Add(1)
		go func(idx int, dr godo.Droplet) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					results[idx] = execResult{name: dr.Name, ip: "?", err: fmt.Errorf("panic: %v", r)}
				}
			}()

			ip := droplet.PublicIPv4(&dr)
			results[idx].name = dr.Name
			results[idx].ip = ip

			if ip == "" {
				results[idx].err = fmt.Errorf("no public IPv4")
				return
			}

			var buf bytes.Buffer
			c := exec.Command(sshBin,
				"-p", "22",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=10",
				fmt.Sprintf("%s@%s", execUser, ip),
				remoteCmd,
			)
			c.Stdout = &buf
			c.Stderr = &buf
			results[idx].err = c.Run()
			results[idx].output = buf.String()
		}(i, d)
	}

	wg.Wait()

	for _, r := range results {
		label := fmt.Sprintf("[%s / %s]", r.name, r.ip)
		if r.err != nil {
			fmt.Printf("%s %s\n%s\n\n",
				color.RedString("✗"), color.WhiteString(label),
				color.RedString("  error: "+r.err.Error()),
			)
		} else {
			fmt.Printf("%s %s\n", color.GreenString("✓"), color.WhiteString(label))
			for _, line := range strings.Split(strings.TrimRight(r.output, "\n"), "\n") {
				fmt.Printf("  %s\n", line)
			}
			fmt.Println()
		}
	}
	return nil
}
