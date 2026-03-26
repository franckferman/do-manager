package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/franckferman/do-manager/pkg/tmpl"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Command tree
// ---------------------------------------------------------------------------

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage built-in cloud-init templates",
	Long: `Built-in cloud-init templates for common Red Team roles.

Templates are bash scripts embedded in the binary. They support variable
substitution via Go's text/template ({{.VarName}} syntax). Unset variables
fall back to declared defaults.

Use --template on 'droplet create' or 'campaign deploy' to apply a template
directly without writing it to a file first. Use --template-var KEY=VALUE to
override default variable values.`,
}

var templateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all available templates",
	RunE:    runTemplateList,
}

var templateShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print the raw (unrendered) content of a template",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateShow,
}

var templateDumpCmd = &cobra.Command{
	Use:   "dump <name>",
	Short: "Render a template with --template-var substitutions and print to stdout",
	Long: `Render a template and print the resulting cloud-init script.
Useful to inspect what will be sent to a Droplet before deploying.

Example:
  do-manager template dump redirector-nginx \
    --template-var C2BackendHost=10.20.0.2 \
    --template-var Domain=cdn.example.com`,
	Args: cobra.ExactArgs(1),
	RunE: runTemplateDump,
}

// ---------------------------------------------------------------------------
// Flags
// ---------------------------------------------------------------------------

var tmplDumpVars []string

func init() {
	templateDumpCmd.Flags().StringArrayVar(&tmplDumpVars, "template-var", nil, "Variable substitution: KEY=VALUE (repeatable)")
	templateCmd.AddCommand(templateListCmd, templateShowCmd, templateDumpCmd)
	rootCmd.AddCommand(templateCmd)
}

// ---------------------------------------------------------------------------
// Runners
// ---------------------------------------------------------------------------

func runTemplateList(cmd *cobra.Command, args []string) error {
	metas, err := tmpl.List()
	if err != nil {
		return err
	}

	if isJSON() {
		return printJSON(metas)
	}

	fmt.Printf("\n%s  %d template(s)\n\n", color.CyanString(">>"), len(metas))

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Name", "Description", "Variables (default)"}
	table.SetHeader(headers)
	cyan := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	colors := make([]tablewriter.Colors, len(headers))
	for i := range colors {
		colors[i] = cyan
	}
	table.SetHeaderColor(colors...)
	table.SetBorder(false)
	table.SetColumnSeparator(" | ")
	table.SetColWidth(40)
	table.SetAutoWrapText(true)

	for _, m := range metas {
		varStrs := make([]string, len(m.Vars))
		for i, v := range m.Vars {
			if v.Default != "" {
				varStrs[i] = v.Name + "=" + v.Default
			} else {
				varStrs[i] = v.Name
			}
		}
		table.Append([]string{
			color.CyanString(m.Name),
			m.Desc,
			strings.Join(varStrs, "\n"),
		})
	}
	table.Render()
	fmt.Printf("\n  %s\n  do-manager template show <name>\n\n",
		color.CyanString("Show full content:"))
	return nil
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	raw, err := tmpl.Raw(args[0])
	if err != nil {
		return err
	}
	fmt.Print(raw)
	return nil
}

func runTemplateDump(cmd *cobra.Command, args []string) error {
	vars := parseTmplVars(tmplDumpVars)
	out, err := tmpl.Render(args[0], vars)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// ---------------------------------------------------------------------------
// Shared helpers (used by droplet and campaign commands)
// ---------------------------------------------------------------------------

// parseTmplVars converts []string{"KEY=VALUE", ...} to map[string]string.
func parseTmplVars(raw []string) map[string]string {
	m := make(map[string]string)
	for _, kv := range raw {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
