package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/franckferman/do-manager/internal/config"
	"github.com/franckferman/do-manager/pkg/client"
	"github.com/franckferman/do-manager/pkg/droplet"
	"github.com/franckferman/do-manager/pkg/firewall"
	"github.com/franckferman/do-manager/pkg/reservedip"
	"github.com/franckferman/do-manager/pkg/vpc"
	"github.com/spf13/cobra"
)

// campaignTag returns the tag used to group all resources of a campaign.
func campaignTag(name string) string {
	return "campaign:" + name
}

// campaignFWName returns the deterministic firewall name for a campaign.
func campaignFWName(name string) string {
	return "campaign-" + name + "-fw"
}

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var campaignCmd = &cobra.Command{
	Use:   "campaign",
	Short: "Orchestrate full Red Team / lab infrastructure",
	Long: `Deploy and destroy complete infrastructure campaigns.

A campaign groups Droplets, a Firewall, and Reserved IPs under a single
name tag. All resources are created in the right order and can be torn
down with a single command.`,
}

var campaignDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a full campaign: Droplets + Firewall + Reserved IPs",
	Long: `Deploy a complete campaign infrastructure.

Example:
  do-manager campaign deploy \
    --name phish-q1 \
    --count 3 \
    --region fra1 \
    --snapshot gophish-v1 \
    --inbound "tcp:443:any" \
    --inbound "tcp:80:any" \
    --inbound "tcp:22:203.0.113.1" \
    --outbound "tcp:0-65535:any" \
    --reserve-ips`,
	RunE: runCampaignDeploy,
}

var campaignStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current state of a campaign",
	RunE:  runCampaignStatus,
}

var campaignDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Teardown all resources belonging to a campaign",
	RunE:  runCampaignDestroy,
}

var campaignRotateIPsCmd = &cobra.Command{
	Use:   "rotate-ips",
	Short: "Replace all reserved IPs of a campaign with fresh ones",
	Long: `Rotate the reserved IPs of a live campaign.

Each reserved IP currently assigned to a campaign Droplet is:
  1. Unassigned from the Droplet
  2. Released back to the pool (deleted)
  3. A new reserved IP is reserved in the same region
  4. The new IP is assigned to the same Droplet

The Droplets keep running throughout. Use this when an IP is burned
(blacklisted, detected, CnC domain seized) and you need a clean IP
without tearing down the whole campaign.

Example:
  do-manager campaign rotate-ips --name phish-q1`,
	RunE: runCampaignRotateIPs,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var (
	campName       string
	campCount      int
	campRegion     string
	campSize       string
	campImage      string
	campSSHKeys    []int
	campTags       []string
	campInbound    []string
	campOutbound   []string
	campReserveIPs   bool
	campForce        bool
	campUserData     string
	campUserDataFile string
	campVPCUUID      string
	campRoles        []string
	campOperatorIP   string
	campVPCAuto      bool
	campTemplateVar  []string
	campPassword     string
)

func init() {
	// deploy
	campaignDeployCmd.Flags().StringVarP(&campName, "name", "n", "", "Campaign name (used as tag and resource prefix)")
	campaignDeployCmd.Flags().IntVarP(&campCount, "count", "c", 1, "Number of Droplets to provision in parallel")
	campaignDeployCmd.Flags().StringVarP(&campRegion, "region", "r", "nyc1", "Region slug")
	campaignDeployCmd.Flags().StringVarP(&campSize, "size", "s", "s-1vcpu-1gb", "Size slug")
	campaignDeployCmd.Flags().StringVarP(&campImage, "snapshot", "i", "ubuntu-22-04-x64", "Image or snapshot slug/ID")
	campaignDeployCmd.Flags().IntSliceVar(&campSSHKeys, "ssh-keys", nil, "SSH key IDs to embed")
	campaignDeployCmd.Flags().StringSliceVar(&campTags, "tags", nil, "Additional tags")
	campaignDeployCmd.Flags().StringArrayVar(&campInbound, "inbound", nil, "Firewall inbound rule: proto:ports:addresses")
	campaignDeployCmd.Flags().StringArrayVar(&campOutbound, "outbound", nil, "Firewall outbound rule: proto:ports:addresses")
	campaignDeployCmd.Flags().BoolVar(&campReserveIPs, "reserve-ips", false, "Reserve and assign a fixed IP to each Droplet")
	campaignDeployCmd.Flags().StringVar(&campUserData, "user-data", "", "Cloud-init / bash startup script (inline)")
	campaignDeployCmd.Flags().StringVar(&campUserDataFile, "user-data-file", "", "Path to a startup script file (run at first boot)")
	campaignDeployCmd.Flags().StringVar(&campVPCUUID, "vpc-uuid", "", "Place all Droplets inside this VPC (use 'do-manager vpc list')")
	campaignDeployCmd.Flags().StringArrayVar(&campRoles, "role", nil, `Role spec: "name:count=N:snapshot=slug:preset=c2[:size=slug][:reserve-ips][:user-data-file=path]" (repeatable)`)
	campaignDeployCmd.Flags().StringVar(&campOperatorIP, "operator-ip", "", "Operator IP used by firewall presets to restrict SSH (required when using --role with a preset)")
	campaignDeployCmd.Flags().BoolVar(&campVPCAuto, "vpc-auto", false, "Automatically create a VPC in the campaign region and attach all Droplets")
	campaignDeployCmd.Flags().StringArrayVar(&campTemplateVar, "template-var", nil, "Template variable: KEY=VALUE (repeatable, applies to all --template or role template=)")
	campaignDeployCmd.Flags().StringVar(&campPassword, "password", "", "Set root password via cloud-init on all Droplets (injected into any template or user-data)")
	campaignDeployCmd.MarkFlagRequired("name") //nolint:errcheck

	// status & destroy
	campaignStatusCmd.Flags().StringVarP(&campName, "name", "n", "", "Campaign name")
	campaignStatusCmd.MarkFlagRequired("name") //nolint:errcheck

	campaignDestroyCmd.Flags().StringVarP(&campName, "name", "n", "", "Campaign name")
	campaignDestroyCmd.Flags().BoolVarP(&campForce, "force", "f", false, "Skip confirmation prompt")
	campaignDestroyCmd.MarkFlagRequired("name") //nolint:errcheck

	campaignRotateIPsCmd.Flags().StringVarP(&campName, "name", "n", "", "Campaign name")
	campaignRotateIPsCmd.MarkFlagRequired("name") //nolint:errcheck

	campaignCmd.AddCommand(campaignDeployCmd, campaignStatusCmd, campaignDestroyCmd, campaignRotateIPsCmd)
	rootCmd.AddCommand(campaignCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newCampaignClients returns initialised service objects.
func newCampaignClients() (*droplet.Service, *firewall.Service, *reservedip.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, nil, nil, err
	}
	return droplet.New(c), firewall.New(c), reservedip.New(c), nil
}

func newCampaignClientsWithVPC() (*droplet.Service, *firewall.Service, *reservedip.Service, *vpc.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return droplet.New(c), firewall.New(c), reservedip.New(c), vpc.New(c), nil
}

// ---------------------------------------------------------------------------
// Role spec parser
// ---------------------------------------------------------------------------

// roleSpec describes one role within a multi-role campaign.
// Format: "name[:count=N][:snapshot=slug][:preset=name][:size=slug][:reserve-ips][:user-data-file=path][:template=name]"
//
// Fields:
//
//	count          number of Droplets for this role (default 1)
//	snapshot       image slug (falls back to campaign --snapshot if omitted)
//	preset         firewall preset: c2 | phishing | redirector | bastion | lockdown
//	size           size slug (falls back to campaign --size if omitted)
//	reserve-ips    reserve and assign one IP per Droplet in this role
//	user-data-file path to a startup script (falls back to campaign --user-data-file if omitted)
//	template       built-in cloud-init template name (see: do-manager template list)
type roleSpec struct {
	Name         string
	Count        int
	Snapshot     string
	Preset       string
	Size         string
	ReserveIPs   bool
	UserDataFile string
	Template     string
}

func parseRoleSpec(s string) (roleSpec, error) {
	parts := strings.Split(s, ":")
	if len(parts) == 0 || parts[0] == "" {
		return roleSpec{}, fmt.Errorf("role spec must start with a name: %q", s)
	}
	r := roleSpec{Name: parts[0], Count: 1}
	for _, kv := range parts[1:] {
		if kv == "reserve-ips" {
			r.ReserveIPs = true
			continue
		}
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			return roleSpec{}, fmt.Errorf("invalid token %q in role %q (expected key=value)", kv, parts[0])
		}
		key, val := kv[:idx], kv[idx+1:]
		switch key {
		case "count":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return roleSpec{}, fmt.Errorf("role %q: count must be a positive integer, got %q", parts[0], val)
			}
			r.Count = n
		case "snapshot", "image":
			r.Snapshot = val
		case "preset":
			r.Preset = val
		case "size":
			r.Size = val
		case "user-data-file":
			r.UserDataFile = val
		case "template":
			r.Template = val
		default:
			return roleSpec{}, fmt.Errorf("role %q: unknown key %q", parts[0], key)
		}
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runCampaignDeploy(cmd *cobra.Command, args []string) error {
	if len(campRoles) > 0 {
		return runCampaignDeployRoles()
	}
	return runCampaignDeploySimple()
}

// runCampaignDeploySimple is the original single-role path (backward compat).
func runCampaignDeploySimple() error {
	tag := campaignTag(campName)
	allTags := append([]string{tag}, campTags...)

	ud, err := resolveCloudInit(campUserData, campUserDataFile, "", parseTmplVars(campTemplateVar))
	if err != nil {
		return err
	}
	if campPassword != "" {
		ud = injectPassword(ud, campPassword)
	}

	dropSvc, fwSvc, ripSvc, err := newCampaignClients()
	if err != nil {
		return err
	}

	start := time.Now()
	step := func(msg string) { fmt.Printf("%s %s\n", color.CyanString(">>"), msg) }
	ok := func(msg string) { fmt.Printf("%s %s\n", color.GreenString("✓"), msg) }

	// ------------------------------------------------------------------ 1. Droplets
	step(fmt.Sprintf("Creating %d Droplet(s) in parallel  [image=%s  region=%s  size=%s]",
		campCount, campImage, campRegion, campSize))

	opts := droplet.CreateOptions{
		Name:     campName,
		Region:   campRegion,
		Size:     campSize,
		Image:    campImage,
		SSHKeys:  campSSHKeys,
		Tags:     allTags,
		UserData: ud,
		VPCUUID:  campVPCUUID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var droplets []int
	var dropletIPs []string

	if campCount == 1 {
		d, err := dropSvc.Create(ctx, opts)
		if err != nil {
			return fmt.Errorf("create droplet: %w", err)
		}
		fmt.Printf("  %s %-20s ID=%d  status=%s\n",
			color.GreenString("✓"), campName, d.ID, statusColor(d.Status))
		droplets = append(droplets, d.ID)

		fmt.Printf("%s Waiting for active state", color.YellowString("~"))
		if err := waitForActive(dropSvc, d.ID); err != nil {
			return err
		}
		fmt.Println()
		refreshCtx, rc := context.WithTimeout(context.Background(), apiTimeout)
		d, err = dropSvc.Get(refreshCtx, d.ID)
		rc()
		if err != nil {
			return err
		}
		dropletIPs = append(dropletIPs, droplet.PublicIPv4(d))
	} else {
		results := dropSvc.CreateBatch(ctx, opts, campCount)
		for _, r := range results {
			if r.Err != nil {
				fmt.Printf("  %s %-20s %s\n", color.RedString("✗"), r.Name, r.Err)
			} else {
				fmt.Printf("  %s %-20s ID=%d  status=%s\n",
					color.GreenString("✓"), r.Name, r.Droplet.ID, statusColor(r.Droplet.Status))
				droplets = append(droplets, r.Droplet.ID)
			}
		}
		if len(droplets) == 0 {
			return fmt.Errorf("all droplets failed to create")
		}

		fmt.Printf("%s Waiting for %d Droplet(s) to become active",
			color.YellowString("~"), len(droplets))
		waitResults := waitForActiveMany(dropSvc, droplets)
		fmt.Println()
		for id, ip := range waitResults {
			dropletIPs = append(dropletIPs, ip)
			if ip == "" {
				fmt.Printf("  %s ID=%-12d timed out\n", color.RedString("✗"), id)
			} else {
				fmt.Printf("  %s ID=%-12d IPv4=%s\n", color.GreenString("✓"), id, color.WhiteString(ip))
			}
		}
	}

	// ------------------------------------------------------------------ 2. Firewall
	if len(campInbound) > 0 || len(campOutbound) > 0 {
		step(fmt.Sprintf("Creating firewall %s", campaignFWName(campName)))

		inRules, err := parseRules(campInbound)
		if err != nil {
			return fmt.Errorf("invalid --inbound: %w", err)
		}
		outRules, err := parseRules(campOutbound)
		if err != nil {
			return fmt.Errorf("invalid --outbound: %w", err)
		}

		fwCtx, fwCancel := context.WithTimeout(context.Background(), 20*time.Second)
		fw, err := fwSvc.Create(fwCtx, firewall.CreateOptions{
			Name:          campaignFWName(campName),
			InboundRules:  inRules,
			OutboundRules: outRules,
			DropletIDs:    droplets,
			Tags:          []string{tag},
		})
		fwCancel()
		if err != nil {
			fmt.Printf("%s Firewall creation failed: %v (continuing)\n", color.YellowString("!"), err)
		} else {
			ok(fmt.Sprintf("Firewall %s created and attached  (in=%d out=%d)",
				fw.Name, len(fw.InboundRules), len(fw.OutboundRules)))
		}
	}

	// ------------------------------------------------------------------ 3. Reserved IPs
	if campReserveIPs && len(droplets) > 0 {
		step(fmt.Sprintf("Reserving %d IP(s) in %s", len(droplets), campRegion))

		reservedIPs := make([]string, 0, len(droplets))
		for i, dID := range droplets {
			ripCtx, ripCancel := context.WithTimeout(context.Background(), 20*time.Second)
			rip, err := ripSvc.Reserve(ripCtx, campRegion, dID)
			ripCancel()
			if err != nil {
				fmt.Printf("  %s Droplet %-12d reserve failed: %v\n", color.RedString("✗"), dID, err)
				continue
			}
			reservedIPs = append(reservedIPs, rip.IP)
			fmt.Printf("  %s Droplet %-12d reserved IP=%s\n",
				color.GreenString("✓"), dID, color.CyanString(rip.IP))
			_ = i
		}
		dropletIPs = reservedIPs
	}

	// ------------------------------------------------------------------ Summary
	elapsed := time.Since(start).Round(time.Second)
	fmt.Printf("\n%s Campaign %s deployed in %s\n",
		color.GreenString("✓"), color.WhiteString(campName), elapsed)
	fmt.Printf("  %-14s %d\n", color.CyanString("Droplets:"), len(droplets))
	fmt.Printf("  %-14s %s\n", color.CyanString("IPs:"), strings.Join(dropletIPs, "  "))
	fmt.Printf("  %-14s %s\n", color.CyanString("Tag:"), tag)
	fmt.Printf("\n  %s\n  %s\n",
		color.CyanString("Teardown when done:"),
		fmt.Sprintf("  do-manager campaign destroy --name %s --force", campName),
	)
	fmt.Println()
	return nil
}

// ---------------------------------------------------------------------------
// Multi-role deploy
// ---------------------------------------------------------------------------

// runCampaignDeployRoles handles `campaign deploy --role ...` deployments.
// Each role gets its own set of Droplets and its own firewall (preset or rules).
// All Droplets share the campaign tag so status/destroy work unchanged.
func runCampaignDeployRoles() error {
	tag := campaignTag(campName)
	allTags := append([]string{tag}, campTags...)
	start := time.Now()
	step := func(msg string) { fmt.Printf("%s %s\n", color.CyanString(">>"), msg) }
	ok := func(msg string) { fmt.Printf("%s %s\n", color.GreenString("✓"), msg) }

	// Parse all role specs upfront so we fail fast on bad input.
	roles := make([]roleSpec, 0, len(campRoles))
	for _, raw := range campRoles {
		r, err := parseRoleSpec(raw)
		if err != nil {
			return fmt.Errorf("--role %q: %w", raw, err)
		}
		roles = append(roles, r)
	}

	dropSvc, fwSvc, ripSvc, vpcSvc, err := newCampaignClientsWithVPC()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// ------------------------------------------------------------------ VPC
	vpcID := campVPCUUID
	if campVPCAuto && vpcID == "" {
		step(fmt.Sprintf("Creating VPC for campaign %s in %s", campName, campRegion))
		v, err := vpcSvc.Create(ctx, vpc.CreateOptions{
			Name:   "campaign-" + campName + "-vpc",
			Region: campRegion,
		})
		if err != nil {
			return fmt.Errorf("create VPC: %w", err)
		}
		vpcID = v.ID
		ok(fmt.Sprintf("VPC created  ID=%s  range=%s", v.ID, v.IPRange))
	}

	// ------------------------------------------------------------------ Roles
	type roleResult struct {
		role       roleSpec
		dropletIDs []int
		ips        []string
	}
	results := make([]roleResult, len(roles))

	totalDroplets := 0
	for ri, role := range roles {
		image := role.Snapshot
		if image == "" {
			image = campImage
		}
		size := role.Size
		if size == "" {
			size = campSize
		}

		ud := ""
		if role.UserDataFile != "" {
			ud, err = resolveCloudInit("", role.UserDataFile, "", parseTmplVars(campTemplateVar))
			if err != nil {
				return fmt.Errorf("role %q: %w", role.Name, err)
			}
		} else if role.Template != "" {
			ud, err = resolveCloudInit("", "", role.Template, parseTmplVars(campTemplateVar))
			if err != nil {
				return fmt.Errorf("role %q: %w", role.Name, err)
			}
		} else {
			ud, err = resolveCloudInit(campUserData, campUserDataFile, "", parseTmplVars(campTemplateVar))
			if err != nil {
				return err
			}
		}
		if campPassword != "" {
			ud = injectPassword(ud, campPassword)
		}

		step(fmt.Sprintf("[%s] Creating %d Droplet(s)  image=%s  size=%s",
			color.CyanString(role.Name), role.Count, image, size))

		opts := droplet.CreateOptions{
			Name:     campName + "-" + role.Name,
			Region:   campRegion,
			Size:     size,
			Image:    image,
			SSHKeys:  campSSHKeys,
			Tags:     append(allTags, "role:"+role.Name),
			UserData: ud,
			VPCUUID:  vpcID,
		}

		var ids []int
		if role.Count == 1 {
			d, err := dropSvc.Create(ctx, opts)
			if err != nil {
				fmt.Printf("  %s [%s] create failed: %v\n", color.RedString("✗"), role.Name, err)
			} else {
				fmt.Printf("  %s %-24s ID=%d\n", color.GreenString("✓"), d.Name, d.ID)
				ids = append(ids, d.ID)
			}
		} else {
			batch := dropSvc.CreateBatch(ctx, opts, role.Count)
			for _, b := range batch {
				if b.Err != nil {
					fmt.Printf("  %s [%s] %s: %v\n", color.RedString("✗"), role.Name, b.Name, b.Err)
				} else {
					fmt.Printf("  %s %-24s ID=%d\n", color.GreenString("✓"), b.Name, b.Droplet.ID)
					ids = append(ids, b.Droplet.ID)
				}
			}
		}

		if len(ids) == 0 {
			return fmt.Errorf("role %q: all Droplets failed to create", role.Name)
		}

		// Wait for active
		fmt.Printf("%s [%s] Waiting for %d Droplet(s)",
			color.YellowString("~"), role.Name, len(ids))
		ipMap := waitForActiveMany(dropSvc, ids)
		fmt.Println()
		var ips []string
		for id, ip := range ipMap {
			if ip == "" {
				fmt.Printf("  %s ID=%d timed out\n", color.RedString("✗"), id)
			} else {
				fmt.Printf("  %s ID=%-12d IPv4=%s\n", color.GreenString("✓"), id, color.WhiteString(ip))
				ips = append(ips, ip)
			}
		}

		results[ri] = roleResult{role: role, dropletIDs: ids, ips: ips}
		totalDroplets += len(ids)

		// ---------------------------------------------------------------- Firewall per role
		fwName := fmt.Sprintf("campaign-%s-%s-fw", campName, role.Name)

		if role.Preset != "" {
			step(fmt.Sprintf("[%s] Applying firewall preset %s", color.CyanString(role.Name), role.Preset))
			inRules, outRules, err := presetRules(role.Preset, campOperatorIP)
			if err != nil {
				fmt.Printf("  %s preset error: %v (skipping firewall)\n", color.YellowString("!"), err)
			} else {
				fwCtx, fwCancel := context.WithTimeout(context.Background(), 20*time.Second)
				fw, err := fwSvc.Create(fwCtx, firewall.CreateOptions{
					Name:          fwName,
					InboundRules:  inRules,
					OutboundRules: outRules,
					DropletIDs:    ids,
					Tags:          []string{tag},
				})
				fwCancel()
				if err != nil {
					fmt.Printf("  %s Firewall failed: %v\n", color.YellowString("!"), err)
				} else {
					ok(fmt.Sprintf("[%s] Firewall %s  (in=%d out=%d)",
						role.Name, fw.Name, len(fw.InboundRules), len(fw.OutboundRules)))
				}
			}
		} else if len(campInbound) > 0 || len(campOutbound) > 0 {
			// Fall back to campaign-level rules
			step(fmt.Sprintf("[%s] Applying campaign firewall rules", color.CyanString(role.Name)))
			inRules, _ := parseRules(campInbound)
			outRules, _ := parseRules(campOutbound)
			fwCtx, fwCancel := context.WithTimeout(context.Background(), 20*time.Second)
			fw, err := fwSvc.Create(fwCtx, firewall.CreateOptions{
				Name:          fwName,
				InboundRules:  inRules,
				OutboundRules: outRules,
				DropletIDs:    ids,
				Tags:          []string{tag},
			})
			fwCancel()
			if err != nil {
				fmt.Printf("  %s Firewall failed: %v\n", color.YellowString("!"), err)
			} else {
				ok(fmt.Sprintf("[%s] Firewall %s  (in=%d out=%d)",
					role.Name, fw.Name, len(fw.InboundRules), len(fw.OutboundRules)))
			}
		}

		// ---------------------------------------------------------------- Reserved IPs
		if role.ReserveIPs {
			step(fmt.Sprintf("[%s] Reserving %d IP(s)", color.CyanString(role.Name), len(ids)))
			var reserved []string
			for _, dID := range ids {
				ripCtx, ripCancel := context.WithTimeout(context.Background(), 20*time.Second)
				rip, err := ripSvc.Reserve(ripCtx, campRegion, dID)
				ripCancel()
				if err != nil {
					fmt.Printf("  %s Droplet %d: %v\n", color.RedString("✗"), dID, err)
					continue
				}
				reserved = append(reserved, rip.IP)
				fmt.Printf("  %s Droplet %-12d IP=%s\n",
					color.GreenString("✓"), dID, color.CyanString(rip.IP))
			}
			results[ri].ips = reserved
		}
	}

	// ------------------------------------------------------------------ Summary
	elapsed := time.Since(start).Round(time.Second)
	fmt.Printf("\n%s Campaign %s deployed in %s  (%d roles  %d droplets)\n",
		color.GreenString("✓"), color.WhiteString(campName), elapsed, len(roles), totalDroplets)
	for _, r := range results {
		fmt.Printf("  %s %-14s droplets=%-4d  ips=%s\n",
			color.CyanString(">>"),
			r.role.Name,
			len(r.dropletIDs),
			strings.Join(r.ips, " "),
		)
	}
	if vpcID != "" {
		fmt.Printf("  %-18s %s\n", color.CyanString("VPC:"), vpcID)
	}
	fmt.Printf("\n  %s\n  do-manager campaign destroy --name %s --force\n\n",
		color.CyanString("Teardown:"), campName)
	return nil
}

func runCampaignStatus(cmd *cobra.Command, args []string) error {
	tag := campaignTag(campName)
	fwName := campaignFWName(campName)

	dropSvc, fwSvc, ripSvc, err := newCampaignClients()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Droplets
	allDroplets, err := dropSvc.List(ctx)
	if err != nil {
		return err
	}
	var campDroplets []int
	var campIPs []string
	for _, d := range allDroplets {
		for _, t := range d.Tags {
			if t == tag {
				campDroplets = append(campDroplets, d.ID)
				campIPs = append(campIPs, droplet.PublicIPv4(&d))
				break
			}
		}
	}

	// Firewall
	allFWs, err := fwSvc.List(ctx)
	if err != nil {
		return err
	}
	var campFW string
	var fwStatus string
	for _, fw := range allFWs {
		if fw.Name == fwName {
			campFW = fw.ID
			fwStatus = fw.Status
			break
		}
	}

	// Reserved IPs
	allIPs, err := ripSvc.List(ctx)
	if err != nil {
		return err
	}
	campDropletSet := make(map[int]bool, len(campDroplets))
	for _, id := range campDroplets {
		campDropletSet[id] = true
	}
	var reservedIPs []string
	for _, rip := range allIPs {
		if rip.Droplet != nil && campDropletSet[rip.Droplet.ID] {
			reservedIPs = append(reservedIPs, rip.IP)
		}
	}

	if isJSON() {
		return printJSON(map[string]any{
			"name":         campName,
			"tag":          tag,
			"droplets":     campDroplets,
			"ips":          campIPs,
			"firewall_id":  campFW,
			"reserved_ips": reservedIPs,
		})
	}

	fmt.Printf("\n%s Campaign %s\n", color.CyanString(">>"), color.WhiteString(campName))
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  %-16s %s\n", color.CyanString("Tag:"), tag)
	fmt.Printf("  %-16s %d active\n", color.CyanString("Droplets:"), len(campDroplets))
	if len(campIPs) > 0 {
		fmt.Printf("  %-16s %s\n", color.CyanString("IPs:"), strings.Join(campIPs, "  "))
	}
	if campFW != "" {
		fmt.Printf("  %-16s %s  (%s)\n", color.CyanString("Firewall:"), fwName, fwStatus)
	} else {
		fmt.Printf("  %-16s %s\n", color.CyanString("Firewall:"), color.YellowString("none"))
	}
	if len(reservedIPs) > 0 {
		fmt.Printf("  %-16s %s\n", color.CyanString("Reserved IPs:"), strings.Join(reservedIPs, "  "))
	}
	fmt.Println()
	return nil
}

func runCampaignDestroy(cmd *cobra.Command, args []string) error {
	tag := campaignTag(campName)
	fwName := campaignFWName(campName)

	if !campForce {
		fmt.Printf("%s Destroy ALL resources for campaign %s? [y/N]: ",
			color.YellowString("!"), color.WhiteString(campName))
		var reply string
		fmt.Scan(&reply)
		if strings.ToLower(strings.TrimSpace(reply)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	dropSvc, fwSvc, ripSvc, err := newCampaignClients()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Find campaign droplets
	allDroplets, err := dropSvc.List(ctx)
	if err != nil {
		return err
	}
	var campDroplets []int
	for _, d := range allDroplets {
		for _, t := range d.Tags {
			if t == tag {
				campDroplets = append(campDroplets, d.ID)
				break
			}
		}
	}

	// Find & release reserved IPs assigned to campaign droplets
	allIPs, err := ripSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list reserved IPs: %w", err)
	}
	campSet := make(map[int]bool, len(campDroplets))
	for _, id := range campDroplets {
		campSet[id] = true
	}
	var releasedIPs int
	for _, rip := range allIPs {
		if rip.Droplet != nil && campSet[rip.Droplet.ID] {
			ripCtx, ripCancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, _ = ripSvc.Unassign(ripCtx, rip.IP)
			delErr := ripSvc.Delete(ripCtx, rip.IP)
			ripCancel()
			if delErr != nil {
				fmt.Printf("  %s Could not release IP %s: %v\n", color.YellowString("!"), rip.IP, delErr)
			} else {
				releasedIPs++
			}
		}
	}

	// Find & delete firewall
	allFWs, err := fwSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list firewalls: %w", err)
	}
	var deletedFW bool
	for _, fw := range allFWs {
		if fw.Name == fwName {
			fwCtx, fwCancel := context.WithTimeout(context.Background(), 15*time.Second)
			err := fwSvc.Delete(fwCtx, fw.ID)
			fwCancel()
			if err != nil {
				fmt.Printf("  %s Could not delete firewall %s: %v\n", color.YellowString("!"), fwName, err)
			} else {
				deletedFW = true
			}
			break
		}
	}

	// Delete droplets
	var deletedDroplets int
	if len(campDroplets) > 0 {
		delCtx, delCancel := context.WithTimeout(context.Background(), 30*time.Second)
		errs := dropSvc.DeleteMany(delCtx, campDroplets)
		delCancel()
		for i, err := range errs {
			if err != nil {
				fmt.Printf("  %s Droplet %d: %v\n", color.RedString("✗"), campDroplets[i], err)
			} else {
				deletedDroplets++
			}
		}
	}

	fmt.Printf("\n%s Campaign %s destroyed\n",
		color.GreenString("✓"), color.WhiteString(campName))
	fmt.Printf("  %-18s %d\n", color.CyanString("Droplets:"), deletedDroplets)
	fmt.Printf("  %-18s %d\n", color.CyanString("Reserved IPs:"), releasedIPs)
	fwStr := "none"
	if deletedFW {
		fwStr = fwName
	}
	fmt.Printf("  %-18s %s\n", color.CyanString("Firewall:"), fwStr)
	fmt.Println()
	return nil
}

func runCampaignRotateIPs(cmd *cobra.Command, args []string) error {
	tag := campaignTag(campName)

	dropSvc, _, ripSvc, err := newCampaignClients()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. Find campaign droplets
	allDroplets, err := dropSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list droplets: %w", err)
	}

	type dropletInfo struct {
		id     int
		name   string
		region string
	}
	var campDroplets []dropletInfo
	for _, d := range allDroplets {
		for _, t := range d.Tags {
			if t == tag {
				campDroplets = append(campDroplets, dropletInfo{
					id:     d.ID,
					name:   d.Name,
					region: d.Region.Slug,
				})
				break
			}
		}
	}

	if len(campDroplets) == 0 {
		return fmt.Errorf("no Droplets found for campaign %q (tag=%s)", campName, tag)
	}

	// 2. Find reserved IPs assigned to those droplets
	allIPs, err := ripSvc.List(ctx)
	if err != nil {
		return fmt.Errorf("list reserved IPs: %w", err)
	}
	campSet := make(map[int]dropletInfo, len(campDroplets))
	for _, d := range campDroplets {
		campSet[d.id] = d
	}

	type rotation struct {
		dropletID   int
		dropletName string
		region      string
		oldIP       string
		newIP       string
	}

	// Build rotation plan: one entry per reserved IP -> droplet pair
	var plan []rotation
	for _, rip := range allIPs {
		if rip.Droplet == nil {
			continue
		}
		if d, ok := campSet[rip.Droplet.ID]; ok {
			plan = append(plan, rotation{
				dropletID:   d.id,
				dropletName: d.name,
				region:      d.region,
				oldIP:       rip.IP,
			})
		}
	}

	if len(plan) == 0 {
		fmt.Printf("%s No reserved IPs found for campaign %q. Nothing to rotate.\n",
			color.YellowString("!"), campName)
		return nil
	}

	fmt.Printf("\n%s Rotating %d IP(s) for campaign %s\n\n",
		color.CyanString(">>"), len(plan), color.WhiteString(campName))

	// 3. For each IP: unassign -> delete -> reserve new -> assign
	for i := range plan {
		p := &plan[i]
		fmt.Printf("  %s %-20s  %s -> ",
			color.CyanString("~"), p.dropletName, color.YellowString(p.oldIP))

		// Unassign
		opCtx, opCancel := context.WithTimeout(context.Background(), 20*time.Second)
		_, unassignErr := ripSvc.Unassign(opCtx, p.oldIP)
		opCancel()
		if unassignErr != nil {
			fmt.Printf("%s (unassign failed: %v)\n", color.RedString("✗"), unassignErr)
			continue
		}

		// Delete old IP
		delCtx, delCancel := context.WithTimeout(context.Background(), 15*time.Second)
		delErr := ripSvc.Delete(delCtx, p.oldIP)
		delCancel()
		if delErr != nil {
			fmt.Printf("%s (delete failed: %v)\n", color.RedString("✗"), delErr)
			continue
		}

		// Reserve new IP in same region, assign immediately to same droplet
		resCtx, resCancel := context.WithTimeout(context.Background(), 20*time.Second)
		newRIP, resErr := ripSvc.Reserve(resCtx, p.region, p.dropletID)
		resCancel()
		if resErr != nil {
			fmt.Printf("%s (reserve failed: %v)\n", color.RedString("✗"), resErr)
			continue
		}

		p.newIP = newRIP.IP
		fmt.Printf("%s\n", color.GreenString(p.newIP))
	}

	fmt.Printf("\n%s Campaign %s IPs rotated\n", color.GreenString("✓"), color.WhiteString(campName))
	fmt.Printf("  Update your DNS records and C2 redirectors to the new IPs above.\n\n")
	return nil
}
