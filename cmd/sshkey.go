package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/franckferman/do-manager/internal/config"
	"github.com/franckferman/do-manager/pkg/client"
	"github.com/franckferman/do-manager/pkg/sshkey"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var sshKeyCmd = &cobra.Command{
	Use:     "ssh-key",
	Aliases: []string{"key", "keys"},
	Short:   "Manage SSH keys",
}

var sshKeyListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all SSH keys in the account",
	RunE:    runSSHKeyList,
}

var sshKeyGetCmd = &cobra.Command{
	Use:   "get <fingerprint>",
	Short: "Show details of an SSH key",
	Args:  cobra.ExactArgs(1),
	RunE:  runSSHKeyGet,
}

var sshKeyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new SSH public key",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if sshKeyPubKey == "" && sshKeyFile == "" {
			return fmt.Errorf("one of --public-key (-k) or --file (-f) is required")
		}
		return nil
	},
	RunE: runSSHKeyAdd,
}

var sshKeyDeleteCmd = &cobra.Command{
	Use:     "delete <fingerprint>",
	Aliases: []string{"rm"},
	Short:   "Delete an SSH key by fingerprint",
	Args:    cobra.ExactArgs(1),
	RunE:    runSSHKeyDelete,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var (
	sshKeyName   string
	sshKeyPubKey string
	sshKeyFile   string
)

func init() {
	sshKeyAddCmd.Flags().StringVarP(&sshKeyName, "name", "n", "", "Key name (required)")
	sshKeyAddCmd.Flags().StringVarP(&sshKeyPubKey, "public-key", "k", "", "Public key content (string)")
	sshKeyAddCmd.Flags().StringVarP(&sshKeyFile, "file", "f", "", "Path to public key file (e.g. ~/.ssh/id_ed25519.pub)")
	sshKeyAddCmd.MarkFlagRequired("name") //nolint:errcheck

	sshKeyCmd.AddCommand(sshKeyListCmd, sshKeyGetCmd, sshKeyAddCmd, sshKeyDeleteCmd)
	rootCmd.AddCommand(sshKeyCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newSSHKeyService() (*sshkey.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg.Token)
	if err != nil {
		return nil, err
	}
	return sshkey.New(c), nil
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runSSHKeyList(cmd *cobra.Command, args []string) error {
	svc, err := newSSHKeyService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	keys, err := svc.List(ctx)
	if err != nil {
		return err
	}
	if isJSON() {
		return printJSON(keys)
	}

	if len(keys) == 0 {
		fmt.Println(color.YellowString("No SSH keys found."))
		return nil
	}

	fmt.Printf("\n%s  %d SSH key(s)\n\n", color.CyanString(">>"), len(keys))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"ID", "Name", "Fingerprint"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	table.SetHeaderColor(cyan, cyan, cyan)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")

	for _, k := range keys {
		table.Append([]string{
			fmt.Sprintf("%d", k.ID),
			k.Name,
			k.Fingerprint,
		})
	}
	table.Render()
	fmt.Println()
	return nil
}

func runSSHKeyGet(cmd *cobra.Command, args []string) error {
	svc, err := newSSHKeyService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	k, err := svc.Get(ctx, args[0])
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(k)
	}

	label := color.CyanString
	fmt.Printf("\n%s SSH Key %s\n\n", color.CyanString(">>"), color.WhiteString(k.Name))
	fmt.Printf("  %-16s %d\n", label("ID:"), k.ID)
	fmt.Printf("  %-16s %s\n", label("Name:"), k.Name)
	fmt.Printf("  %-16s %s\n", label("Fingerprint:"), k.Fingerprint)
	fmt.Println()
	return nil
}

func runSSHKeyAdd(cmd *cobra.Command, args []string) error {
	pubKey := sshKeyPubKey
	if pubKey == "" && sshKeyFile != "" {
		data, err := os.ReadFile(sshKeyFile)
		if err != nil {
			return fmt.Errorf("read key file %q: %w", sshKeyFile, err)
		}
		pubKey = string(data)
	}
	if pubKey == "" {
		return fmt.Errorf("provide --public-key or --file")
	}

	svc, err := newSSHKeyService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	k, err := svc.Create(ctx, sshKeyName, pubKey)
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(k)
	}
	fmt.Printf("%s SSH key added  ID=%d  Fingerprint=%s\n",
		color.GreenString("✓"),
		k.ID,
		k.Fingerprint,
	)
	return nil
}

func runSSHKeyDelete(cmd *cobra.Command, args []string) error {
	svc, err := newSSHKeyService()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.Delete(ctx, args[0]); err != nil {
		return err
	}

	fmt.Printf("%s SSH key %s deleted.\n",
		color.GreenString("✓"),
		color.WhiteString(args[0]),
	)
	return nil
}
