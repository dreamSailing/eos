package render

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/muesli/termenv"
)

func HighlightCodeANSI(code string, lang string) string {
	code = strings.ReplaceAll(code, "\r\n", "\n")
	code = strings.TrimRight(code, "\n")
	if code == "" {
		return ""
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "js":
		lang = "javascript"
	case "ts":
		lang = "typescript"
	case "py":
		lang = "python"
	case "sh":
		lang = "bash"
	case "yml":
		lang = "yaml"
	}

	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	formatterName := "terminal16"
	switch termenv.EnvColorProfile() {
	case termenv.ANSI256:
		formatterName = "terminal256"
	case termenv.TrueColor:
		formatterName = "terminal16m"
	}
	formatter := formatters.Get(formatterName)
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, it); err != nil {
		return code
	}
	return strings.TrimRight(buf.String(), "\n")
}
