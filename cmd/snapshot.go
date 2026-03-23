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
	"github.com/franckferman/do-manager/pkg/snapshot"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var snapshotCmd = &cobra.Command{
	Use:     "snapshot",
	Aliases: []string{"snap"},
	Short:   "Manage Droplet snapshots",
}

var snapshotListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all Droplet snapshots",
	RunE:    runSnapshotList,
}

var snapshotGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show details of a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnapshotGet,
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a snapshot from a running Droplet",
	Long: `Create a snapshot of a Droplet.

The snapshot can later be used as an image slug with:
  do-manager droplet create --image <snapshot-id> ...

Use --wait to block until the snapshot action completes.`,
	RunE: runSnapshotCreate,
}

var snapshotDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a snapshot",
	Args:    cobra.ExactArgs(1),
	RunE:    runSnapshotDelete,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var (
	snapDropletID int
	snapName      string
	snapWait      bool
)

func init() {
	snapshotCreateCmd.Flags().IntVarP(&snapDropletID, "droplet", "d", 0, "Source Droplet ID")
	snapshotCreateCmd.Flags().StringVarP(&snapName, "name", "n", "", "Snapshot name")
	snapshotCreateCmd.Flags().BoolVarP(&snapWait, "wait", "w", false, "Wait until snapshot action completes")
	snapshotCreateCmd.MarkFlagRequired("droplet") //nolint:errcheck
	snapshotCreateCmd.MarkFlagRequired("name")    //nolint:errcheck

	snapshotCmd.AddCommand(
		snapshotListCmd,
		snapshotGetCmd,
		snapshotCreateCmd,
		snapshotDeleteCmd,
	)
	rootCmd.AddCommand(snapshotCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newSnapshotService() (*snapshot.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return snapshot.New(c), nil
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runSnapshotList(cmd *cobra.Command, args []string) error {
	svc, err := newSnapshotService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	snaps, err := svc.List(ctx)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(snaps)
	}

	if len(snaps) == 0 {
		fmt.Println(color.YellowString("No snapshots found."))
		return nil
	}

	fmt.Printf("\n%s  %d snapshot(s)\n\n", color.CyanString(">>"), len(snaps))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"ID", "Name", "Resource ID", "Regions", "Size (GB)", "Created"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, s := range snaps {
		regionStr := "-"
		if len(s.Regions) > 0 {
			regionStr = s.Regions[0]
			if len(s.Regions) > 1 {
				regionStr += fmt.Sprintf(" +%d", len(s.Regions)-1)
			}
		}
		table.Append([]string{
			s.ID,
			s.Name,
			s.ResourceID,
			regionStr,
			fmt.Sprintf("%.1f", s.SizeGigaBytes),
			s.Created,
		})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runSnapshotGet(cmd *cobra.Command, args []string) error {
	svc, err := newSnapshotService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s, err := svc.Get(ctx, args[0])
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(s)
	}

	label := color.CyanString
	fmt.Printf("\n%s Snapshot %s\n\n", color.CyanString(">>"), color.WhiteString(s.Name))
	fmt.Printf("  %-16s %s\n", label("ID:"), s.ID)
	fmt.Printf("  %-16s %s\n", label("Name:"), s.Name)
	fmt.Printf("  %-16s %s (type=%s)\n", label("Resource:"), s.ResourceID, s.ResourceType)
	fmt.Printf("  %-16s %s\n", label("Regions:"), strconv.Itoa(len(s.Regions)))
	fmt.Printf("  %-16s %.1f GB\n", label("Size:"), s.SizeGigaBytes)
	fmt.Printf("  %-16s %d GB\n", label("Min Disk:"), s.MinDiskSize)
	fmt.Printf("  %-16s %s\n", label("Created:"), s.Created)
	fmt.Printf("\n  %s\n  %s\n",
		label("Deploy from this snapshot:"),
		fmt.Sprintf("  do-manager droplet create --image %s --region %s --wait",
			s.ID, func() string {
				if len(s.Regions) > 0 {
					return s.Regions[0]
				}
				return "<region>"
			}()),
	)
	fmt.Println()
	return nil
}

func runSnapshotCreate(cmd *cobra.Command, args []string) error {
	svc, err := newSnapshotService()
	if err != nil {
		return err
	}

	if !isJSON() {
		fmt.Printf("%s Snapshotting droplet %s as %s",
			color.CyanString(">>"),
			color.WhiteString(strconv.Itoa(snapDropletID)),
			color.WhiteString(snapName),
		)
		if snapWait {
			fmt.Print(" (waiting for completion)")
		}
		fmt.Println()
	}

	ctx := context.Background()
	if !snapWait {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}

	action, err := svc.CreateFromDroplet(ctx, snapDropletID, snapName, snapWait)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(action)
	}

	if snapWait {
		fmt.Printf("%s Snapshot complete  action_id=%d\n",
			color.GreenString("✓"), action.ID)
		fmt.Printf("  %s\n  %s\n",
			color.CyanString("To deploy from this snapshot:"),
			fmt.Sprintf("  do-manager snapshot list -o json | jq '.[] | select(.name==\"%s\") | .id'", snapName),
		)
	} else {
		fmt.Printf("%s Snapshot action initiated  action_id=%d  status=%s\n",
			color.GreenString("✓"),
			action.ID,
			color.YellowString(action.Status),
		)
		fmt.Printf("  %s\n", color.YellowString("Use --wait to block until complete, or check status via the API."))
	}
	return nil
}

func runSnapshotDelete(cmd *cobra.Command, args []string) error {
	svc, err := newSnapshotService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.Delete(ctx, args[0]); err != nil {
		return err
	}
	fmt.Printf("%s Snapshot %s deleted.\n", color.GreenString("✓"), color.WhiteString(args[0]))
	return nil
}
