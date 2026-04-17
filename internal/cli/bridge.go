package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/serve"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
	"github.com/spf13/cobra"
)

func newBridgeCmd() *cobra.Command {
	var transport string
	var workspace string
	var allowedTools string
	var policyPath string
	var outputPath string
	var launchCommand string
	var requireApprovalDigest bool
	var includeTools bool
	var includeCapabilities bool

	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Generate bridge metadata for IDE and remote hosts.",
	}

	manifestCmd := &cobra.Command{
		Use:   "manifest",
		Short: "Print a JSON bridge manifest for eos serve.",
		RunE: func(cmd *cobra.Command, args []string) error {
			transport = strings.TrimSpace(transport)
			if transport == "" {
				transport = "stdio"
			}
			if transport != "stdio" {
				return fmt.Errorf("unsupported transport: %s", transport)
			}
			if strings.TrimSpace(workspace) == "" {
				return fmt.Errorf("workspace required")
			}

			manifest, err := serve.BuildBridgeManifest(serve.Options{
				Transport:             transport,
				DefaultWorkspacePath:  workspace,
				DefaultAllowedTools:   splitCommaList(allowedTools),
				PolicyPath:            policyPath,
				RequireApprovalDigest: requireApprovalDigest,
			}, serve.BridgeManifestOptions{
				LaunchCommand:       strings.TrimSpace(launchCommand),
				Services:            toolapiimpl.NewServices(),
				IncludeTools:        includeTools,
				IncludeCapabilities: includeCapabilities,
			})
			if err != nil {
				return err
			}

			bs, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return err
			}
			bs = append(bs, '\n')

			outputPath = strings.TrimSpace(outputPath)
			if outputPath == "" {
				_, err = cmd.OutOrStdout().Write(bs)
				return err
			}
			if !filepath.IsAbs(outputPath) {
				if abs, err := filepath.Abs(outputPath); err == nil {
					outputPath = abs
				}
			}
			if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
				return err
			}
			return os.WriteFile(outputPath, bs, 0o644)
		},
	}

	manifestCmd.Flags().StringVar(&transport, "transport", "stdio", "bridge transport: stdio")
	manifestCmd.Flags().StringVar(&workspace, "workspace", "", "workspace path (required)")
	manifestCmd.Flags().StringVar(&allowedTools, "allowed-tools", "", "comma-separated allowed tools (optional)")
	manifestCmd.Flags().StringVar(&policyPath, "policy", "", "policy json file path (optional)")
	manifestCmd.Flags().BoolVar(&requireApprovalDigest, "require-approval-digest", true, "require approvalDigest for medium/high risk tools")
	manifestCmd.Flags().StringVar(&launchCommand, "command", "", "launch command for hosts (defaults to current executable)")
	manifestCmd.Flags().StringVar(&outputPath, "out", "", "optional output file path")
	manifestCmd.Flags().BoolVar(&includeTools, "include-tools", true, "embed executable tool catalog")
	manifestCmd.Flags().BoolVar(&includeCapabilities, "include-capabilities", true, "embed capability catalog")
	_ = manifestCmd.MarkFlagRequired("workspace")

	cmd.AddCommand(manifestCmd)
	return cmd
}
