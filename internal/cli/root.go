package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/dreamSailing/vb-coding/internal/ui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootLang 根据环境变量 VB_LANG 返回界面语言。
// 当前仅支持 zh/en，默认返回 zh。
func rootLang() string {
	lang := os.Getenv("VB_LANG")
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
	Use:   "vb-coding",
	Short: rootShort(),
	Long:  rootLong(),
	Run: func(cmd *cobra.Command, args []string) {
		slog.Info("cli.start", "lang", rootLang())
		ui.StartInteractiveTUI()
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.vb-coding.yaml)")

	rootCmd.AddCommand(newServeCmd())
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
		viper.SetConfigName(".vb-coding")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		slog.Info("cli.config.loaded", "path", viper.ConfigFileUsed())
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		slog.Debug("cli.config.not_loaded", "error", err.Error())
	}
}
