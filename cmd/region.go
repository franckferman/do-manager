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
	"github.com/franckferman/do-manager/pkg/region"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var regionCmd = &cobra.Command{
	Use:   "region",
	Short: "List available DigitalOcean regions",
}

var regionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all regions",
	RunE:    runRegionList,
}

var sizeCmd = &cobra.Command{
	Use:   "size",
	Short: "List available Droplet sizes",
}

var sizeListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all Droplet sizes",
	RunE:    runSizeList,
}

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "List available images",
}

var imageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List images",
	RunE:    runImageList,
}

var imageTypeFilter string

func init() {
	imageListCmd.Flags().StringVarP(&imageTypeFilter, "type", "t", "distribution",
		"Image type filter: distribution | application | user (empty = all)")

	regionCmd.AddCommand(regionListCmd)
	sizeCmd.AddCommand(sizeListCmd)
	imageCmd.AddCommand(imageListCmd)

	rootCmd.AddCommand(regionCmd, sizeCmd, imageCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newRegionService() (*region.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return region.New(c), nil
}

func cyanHeaders(table *tablewriter.Table, headers []string) {
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeader(headers)
	table.SetHeaderColor(colors...)
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runRegionList(cmd *cobra.Command, args []string) error {
	svc, err := newRegionService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	regions, err := svc.ListRegions(ctx)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(regions)
	}

	fmt.Printf("\n%s  %d region(s)\n\n", color.CyanString(">>"), len(regions))

	table := tablewriter.NewWriter(os.Stdout)
	cyanHeaders(table, []string{"Slug", "Name", "Available"})
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, r := range regions {
		avail := color.GreenString("yes")
		if !r.Available {
			avail = color.RedString("no")
		}
		table.Append([]string{r.Slug, r.Name, avail})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runSizeList(cmd *cobra.Command, args []string) error {
	svc, err := newRegionService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sizes, err := svc.ListSizes(ctx)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(sizes)
	}

	fmt.Printf("\n%s  %d size(s)\n\n", color.CyanString(">>"), len(sizes))

	table := tablewriter.NewWriter(os.Stdout)
	cyanHeaders(table, []string{"Slug", "vCPUs", "Memory (MB)", "Disk (GB)", "Transfer (TB)", "Price/mo", "Available", "Regions"})
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, s := range sizes {
		avail := color.GreenString("yes")
		if !s.Available {
			avail = color.RedString("no")
		}
		table.Append([]string{
			s.Slug,
			strconv.Itoa(s.Vcpus),
			strconv.Itoa(s.Memory),
			strconv.Itoa(s.Disk),
			fmt.Sprintf("%.1f", s.Transfer),
			fmt.Sprintf("$%.2f", s.PriceMonthly),
			avail,
			strconv.Itoa(len(s.Regions)),
		})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runImageList(cmd *cobra.Command, args []string) error {
	svc, err := newRegionService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	images, err := svc.ListImages(ctx, imageTypeFilter)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(images)
	}

	typeLabel := imageTypeFilter
	if typeLabel == "" {
		typeLabel = "all"
	}
	fmt.Printf("\n%s  %d image(s)  [type=%s]\n\n", color.CyanString(">>"), len(images), typeLabel)

	table := tablewriter.NewWriter(os.Stdout)
	cyanHeaders(table, []string{"ID", "Slug", "Name", "Distribution", "Min Disk"})
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, img := range images {
		table.Append([]string{
			strconv.Itoa(img.ID),
			img.Slug,
			img.Name,
			img.Distribution,
			strconv.Itoa(img.MinDiskSize) + " GB",
		})
	}
	table.Render()
	fmt.Println()
	return nil
}
