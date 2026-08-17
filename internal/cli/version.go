package cli

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"fmt"
	"runtime"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	cfg, _ := config.Load()
	return &cobra.Command{
		Use:   "version",
		Short: i18n.T("version.short", cfg.Language),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("eos %s (%s/%s, %s)\n",
				version.AppVersion, runtime.GOOS, runtime.GOARCH, runtime.Version())
			if version.BuildCommit != "" && version.BuildCommit != "unknown" {
				fmt.Printf("commit %s, built %s\n", version.BuildCommit, version.BuildDate)
			}
		},
	}
}
