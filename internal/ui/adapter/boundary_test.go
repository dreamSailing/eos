package adapter

import (
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestImportBoundary(t *testing.T) {
	forbidden := map[string]string{
		"internal/tools/git":  "adapter 禁止直接依赖 internal/tools/git，请改用 coreapi.GitService (a.engine.Git())",
		"internal/tools/file": "adapter 禁止直接依赖 internal/tools/fileops 生产代码，请改用 coreapi 版本方法",
	}

	forbiddenPrefixes := []struct {
		prefix string
		msg    string
	}{
		{"github.com/dreamSailing/eos/internal/tools", "adapter 禁止直接依赖 internal/tools，请改用 coreapi.Engine"},
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, wd, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") &&
			!strings.HasSuffix(fi.Name(), ".tmp")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parser.ParseDir() error = %v", err)
	}

	for _, pkg := range pkgs {
		for filePath, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)

				if msg, ok := forbidden[path]; ok {
					t.Errorf("%s: forbidden import %q — %s", filePath, path, msg)
				}

				for _, fp := range forbiddenPrefixes {
					if strings.HasPrefix(path, fp.prefix) {
						if _, isExact := forbidden[path]; isExact {
							continue
						}
						t.Errorf("%s: forbidden import %q — %s", filePath, path, fp.msg)
					}
				}
			}
		}
	}
}

func TestAllowedImports(t *testing.T) {
	allowed := map[string]bool{
		"github.com/dreamSailing/eos/internal/bridge":           true,
		"github.com/dreamSailing/eos/internal/config":           true,
		"github.com/dreamSailing/eos/internal/pkg/settings":     true,
		"github.com/dreamSailing/eos/internal/pkg/workspace":    true,
		"github.com/dreamSailing/eos/internal/session":          true,
		"github.com/dreamSailing/eos/pkg/core":                  true,
		"github.com/dreamSailing/eos/pkg/coreapi":               true,
		"github.com/dreamSailing/eos/pkg/coreapi/jsonrpc":       true,
		"github.com/dreamSailing/eos/pkg/protocol":              true,
		"github.com/dreamSailing/eos/pkg/protocol/jsonrpc":      true,
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, wd, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") &&
			!strings.HasSuffix(fi.Name(), ".tmp")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parser.ParseDir() error = %v", err)
	}

	for _, pkg := range pkgs {
		for filePath, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.HasPrefix(path, "github.com/dreamSailing/eos/internal/") ||
					strings.HasPrefix(path, "github.com/dreamSailing/eos/pkg/") {
					if !allowed[path] {
						t.Errorf("%s: unexpected internal/pkg import %q not in allowlist — if legitimate, update TestAllowedImports", filePath, path)
					}
				}
			}
		}
	}
}

func TestCoreFallbackContraction(t *testing.T) {
	migratedChecks := []struct {
		methodName     string
		primaryToken   string
		fallbackToken  string
		primaryBefore  string
		msg            string
	}{
		{
			methodName: "GetModelInfo", primaryToken: "a.runtime.ListModelDescriptors()",
			fallbackToken: "a.core.ModelName()", primaryBefore: "a.core.ModelName()",
			msg: "GetModelInfo 主路径应通过 a.runtime.ListModelDescriptors()，a.core 仅作 fallback",
		},
		{
			methodName: "ResolveAPIConfig", primaryToken: "a.runtime.ListModelDescriptors()",
			fallbackToken: "a.core.ResolveAPIConfig()", primaryBefore: "a.core.ResolveAPIConfig()",
			msg: "ResolveAPIConfig 主路径应通过 a.runtime.ListModelDescriptors()，a.core 仅作 fallback",
		},
		{
			methodName: "CurrentContextUsage", primaryToken: "a.runtime.ContextStats()",
			fallbackToken: "cm.GetCurrentUsage()", primaryBefore: "",
			msg: "CurrentContextUsage 不应再直接调用 a.core.GetContext().GetCurrentUsage()",
		},
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	raw, err := os.ReadFile(wd + string(os.PathSeparator) + "runtime.go")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	content := string(raw)

	for _, mc := range migratedChecks {
		primaryIdx := strings.Index(content, mc.primaryToken)
		if primaryIdx < 0 {
			t.Errorf("%s: 未找到主路径 token %q — 迁移可能被意外移除", mc.methodName, mc.primaryToken)
			continue
		}
		if mc.primaryBefore != "" {
			fallbackIdx := strings.Index(content, mc.fallbackToken)
			if fallbackIdx >= 0 && fallbackIdx < primaryIdx {
				t.Errorf("%s: fallback token %q 出现在主路径 token %q 之前 — %s", mc.methodName, mc.fallbackToken, mc.primaryToken, mc.msg)
			}
		} else {
			re, err := regexp.Compile(regexp.QuoteMeta(mc.fallbackToken))
			if err != nil {
				t.Fatalf("invalid pattern %q: %v", mc.fallbackToken, err)
			}
			if locs := re.FindAllStringIndex(content, -1); len(locs) > 0 {
				for _, loc := range locs {
					line := strings.Count(content[:loc[0]], "\n") + 1
					t.Errorf("runtime.go:%d: %s — %s", line, mc.fallbackToken, mc.msg)
				}
			}
		}
	}
}
