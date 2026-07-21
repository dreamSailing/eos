package cli

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dreamSailing/eos/internal/ui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var (
	printQuery         string
	outputFormat       string
	continueChat       bool
	resumeSession      string
	cliModel           string
	cliMaxTurns        int
	cliAllowedTools    string
	cliDisallowedTools string
	cliAccessMode      string
	cliApprovalMode    string
	cliSandboxMode     string
	cliSkipPermissions bool
)

// rootLang 根据环境变量 EOS_LANG 返回界面语言。
// 当前仅支持 zh/en，默认返回 zh。
func rootLang() string {
	lang := os.Getenv("EOS_LANG")
	if lang == "en" {
		return "en"
	}
	if lang == "zh" {
		return "zh"
	}
	return "zh"
}

// rootShort 返回根命令的短描述（用于 help 等场景）。
func rootShort() string {
	return ui.T("cli.root.short", rootLang())
}

// rootLong 返回根命令的长描述（用于 help 等场景）。
func rootLong() string {
	return ui.T("cli.root.long", rootLang())
}

// rootCmd 表示根命令：不带子命令时启动交互式 TUI。
var rootCmd = &cobra.Command{
	Use:   "eos",
	Short: rootShort(),
	Long:  rootLong(),
	Run: func(cmd *cobra.Command, args []string) {
		slog.Info("cli.start", "lang", rootLang())

		// Handle --print mode
		if printQuery != "" {
			modes := resolveModeConfig(cliAccessMode, cliApprovalMode, cliSandboxMode, cliSkipPermissions)
			if err := RunPrintMode(PrintOptions{
				Query:           printQuery,
				OutputFormat:    outputFormat,
				AccessMode:      modes.AccessMode,
				ApprovalMode:    modes.ApprovalMode,
				SandboxMode:     modes.SandboxMode,
				SkipPermissions: modes.SkipAllChecks,
			}); err != nil {
				os.Exit(1)
			}
			return
		}

		// Build TUI options from CLI flags
		opts := ui.TUIOptions{
			ModelOverride:   cliModel,
			MaxTurns:        cliMaxTurns,
			AccessMode:      cliAccessMode,
			ApprovalMode:    cliApprovalMode,
			SandboxMode:     cliSandboxMode,
			SkipPermissions: cliSkipPermissions,
		}
		if continueChat {
			opts.SessionID = "latest"
		}
		if resumeSession != "" {
			opts.SessionID = resumeSession
		}
		if cliAllowedTools != "" {
			for _, t := range strings.Split(cliAllowedTools, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					opts.AllowedTools = append(opts.AllowedTools, t)
				}
			}
		}
		if cliDisallowedTools != "" {
			for _, t := range strings.Split(cliDisallowedTools, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					opts.DisallowedTools = append(opts.DisallowedTools, t)
				}
			}
		}

		ui.StartInteractiveTUIWithOptions(opts)
	},
}

// Execute 执行根命令入口（由 main 调用）。
func Execute() error {
	slog.Info("cli.execute")
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// 全局 flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.eos.yaml)")
	rootCmd.PersistentFlags().StringVarP(&printQuery, "print", "p", "", "Run a single query in headless mode and print the result")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output-format", "text", "Output format for print mode: text, json, stream-json")
	rootCmd.PersistentFlags().BoolVarP(&continueChat, "continue", "c", false, "Continue the most recent conversation")
	rootCmd.PersistentFlags().StringVar(&resumeSession, "resume", "", "Resume a specific conversation by session ID")
	rootCmd.PersistentFlags().StringVar(&cliModel, "model", "", "Override the model for this session")
	rootCmd.PersistentFlags().IntVar(&cliMaxTurns, "max-turns", 0, "Maximum number of turns (0=unlimited)")
	rootCmd.PersistentFlags().StringVar(&cliAllowedTools, "allowed-tools", "", "Comma-separated list of allowed tools")
	rootCmd.PersistentFlags().StringVar(&cliDisallowedTools, "disallowed-tools", "", "Comma-separated list of disallowed tools")
	rootCmd.PersistentFlags().StringVar(&cliAccessMode, "access-mode", "", "Access mode: read-only, workspace-write, or danger-full-access")
	rootCmd.PersistentFlags().StringVar(&cliApprovalMode, "approval-mode", "", "Approval mode: untrusted, on-failure, on-request, or never")
	rootCmd.PersistentFlags().StringVar(&cliSandboxMode, "sandbox-mode", "workspace", "Legacy sandbox mode alias: workspace or full_access")
	rootCmd.PersistentFlags().BoolVar(&cliSkipPermissions, "dangerously-skip-permissions", false, "Compatibility alias for --access-mode danger-full-access --approval-mode never")

	rootCmd.AddCommand(newDocumentCmd())
	rootCmd.AddCommand(newExecCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newHiddenLegalCmd())
}

// initConfig 读取配置文件与环境变量。
func initConfig() {
	if cfgFile != "" {
		slog.Info("cli.config.file_set", "path", cfgFile)
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		slog.Debug("cli.config.search", "home", home)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".eos")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		slog.Info("cli.config.loaded", "path", viper.ConfigFileUsed())
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		slog.Debug("cli.config.not_loaded", "error", err.Error())
	}
}
