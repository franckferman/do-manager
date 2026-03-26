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
	"github.com/franckferman/do-manager/pkg/dns"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Manage domains and DNS records",
}

var dnsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all domains",
	RunE:    runDNSList,
}

var dnsGetCmd = &cobra.Command{
	Use:   "get <domain>",
	Short: "Show details of a domain",
	Args:  cobra.ExactArgs(1),
	RunE:  runDNSGet,
}

var dnsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Register a new domain",
	RunE:  runDNSCreate,
}

var dnsDeleteCmd = &cobra.Command{
	Use:     "delete <domain>",
	Aliases: []string{"rm"},
	Short:   "Delete a domain and all its records",
	Args:    cobra.ExactArgs(1),
	RunE:    runDNSDelete,
}

var dnsRecordsCmd = &cobra.Command{
	Use:   "records <domain>",
	Short: "List DNS records for a domain",
	Args:  cobra.ExactArgs(1),
	RunE:  runDNSRecords,
}

var dnsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a DNS record",
	Long: `Add a DNS record to a domain.

Examples:
  # Point @ to a Droplet IP (A record)
  do-manager dns add --domain evil.com --type A --name @ --data 1.2.3.4

  # Add a CNAME
  do-manager dns add --domain evil.com --type CNAME --name mail --data @

  # Add TXT for SPF / verification
  do-manager dns add --domain evil.com --type TXT --name @ --data "v=spf1 mx -all"`,
	RunE: runDNSAdd,
}

var dnsRmRecordCmd = &cobra.Command{
	Use:   "rm-record <domain> <record-id>",
	Short: "Delete a DNS record by ID",
	Args:  cobra.ExactArgs(2),
	RunE:  runDNSRmRecord,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var (
	dnsDomain     string
	dnsIP         string
	dnsRecordType string
	dnsName       string
	dnsData       string
	dnsTTL        int
)

func init() {
	dnsCreateCmd.Flags().StringVarP(&dnsDomain, "domain", "d", "", "Domain name (e.g. example.com)")
	dnsCreateCmd.Flags().StringVar(&dnsIP, "ip", "", "Optional: create default A record pointing to this IP")
	dnsCreateCmd.MarkFlagRequired("domain") //nolint:errcheck

	dnsAddCmd.Flags().StringVarP(&dnsDomain, "domain", "d", "", "Domain name")
	dnsAddCmd.Flags().StringVarP(&dnsRecordType, "type", "t", "A", "Record type: A, AAAA, CNAME, MX, TXT, NS, SRV, CAA")
	dnsAddCmd.Flags().StringVarP(&dnsName, "name", "n", "@", "Record name (@ for root, subdomain for sub)")
	dnsAddCmd.Flags().StringVar(&dnsData, "data", "", "Record data / value (IP, hostname, text)")
	dnsAddCmd.Flags().IntVar(&dnsTTL, "ttl", 1800, "TTL in seconds")
	dnsAddCmd.MarkFlagRequired("domain") //nolint:errcheck
	dnsAddCmd.MarkFlagRequired("data")   //nolint:errcheck

	dnsCmd.AddCommand(
		dnsListCmd,
		dnsGetCmd,
		dnsCreateCmd,
		dnsDeleteCmd,
		dnsRecordsCmd,
		dnsAddCmd,
		dnsRmRecordCmd,
	)
	rootCmd.AddCommand(dnsCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newDNSService() (*dns.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return dns.New(c), nil
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runDNSList(cmd *cobra.Command, args []string) error {
	svc, err := newDNSService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	domains, err := svc.ListDomains(ctx)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(domains)
	}

	if len(domains) == 0 {
		fmt.Println(color.YellowString("No domains found."))
		return nil
	}

	fmt.Printf("\n%s  %d domain(s)\n\n", color.CyanString(">>"), len(domains))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Domain", "TTL"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, d := range domains {
		table.Append([]string{d.Name, strconv.Itoa(d.TTL)})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runDNSGet(cmd *cobra.Command, args []string) error {
	svc, err := newDNSService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	d, err := svc.GetDomain(ctx, args[0])
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(d)
	}

	label := color.CyanString
	fmt.Printf("\n%s Domain %s\n\n", color.CyanString(">>"), color.WhiteString(d.Name))
	fmt.Printf("  %-10s %s\n", label("Name:"), d.Name)
	fmt.Printf("  %-10s %d\n", label("TTL:"), d.TTL)
	fmt.Println()
	return nil
}

func runDNSCreate(cmd *cobra.Command, args []string) error {
	svc, err := newDNSService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	d, err := svc.CreateDomain(ctx, dnsDomain, dnsIP)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(d)
	}

	fmt.Printf("%s Domain %s registered\n", color.GreenString("✓"), color.WhiteString(d.Name))
	return nil
}

func runDNSDelete(cmd *cobra.Command, args []string) error {
	svc, err := newDNSService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.DeleteDomain(ctx, args[0]); err != nil {
		return err
	}
	fmt.Printf("%s Domain %s deleted.\n", color.GreenString("✓"), color.WhiteString(args[0]))
	return nil
}

func runDNSRecords(cmd *cobra.Command, args []string) error {
	svc, err := newDNSService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	records, err := svc.ListRecords(ctx, args[0])
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(records)
	}

	if len(records) == 0 {
		fmt.Println(color.YellowString("No records found."))
		return nil
	}

	fmt.Printf("\n%s  %d record(s) for %s\n\n",
		color.CyanString(">>"), len(records), color.WhiteString(args[0]))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"ID", "Type", "Name", "Data", "TTL"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, r := range records {
		data := r.Data
		if len(data) > 40 {
			data = data[:37] + "..."
		}
		table.Append([]string{
			strconv.Itoa(r.ID),
			color.CyanString(r.Type),
			r.Name,
			data,
			strconv.Itoa(r.TTL),
		})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runDNSAdd(cmd *cobra.Command, args []string) error {
	svc, err := newDNSService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	r, err := svc.CreateRecord(ctx, dnsDomain, dnsRecordType, dnsName, dnsData, dnsTTL)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(r)
	}

	fmt.Printf("%s Record created  ID=%d  %s %s -> %s  TTL=%d\n",
		color.GreenString("✓"),
		r.ID,
		color.CyanString(r.Type),
		r.Name,
		r.Data,
		r.TTL,
	)
	return nil
}

func runDNSRmRecord(cmd *cobra.Command, args []string) error {
	recordID, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid record ID %q", args[1])
	}
	svc, err := newDNSService()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.DeleteRecord(ctx, args[0], recordID); err != nil {
		return err
	}
	fmt.Printf("%s Record %d deleted from %s\n",
		color.GreenString("✓"), recordID, color.WhiteString(args[0]))
	return nil
}
