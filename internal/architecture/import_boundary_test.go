// Package architecture 守护 EOS 架构边界，防止迁移过程中耦合回退。
//
// 本文件定义了三层导入边界守卫：
//
//  1. TestUIDirectRuntimeCouplingDoesNotSpread — 阻止 internal/ui 下所有包（含 adapter）
//     导入 internal/bridge、internal/runtime、internal/tools、pkg/core。
//     adapter 不再是兼容性孤岛；唯一允许的 core 入口是 pkg/coreapi/sidecar/client facade。
//
//  2. TestNewCorePackagesDoNotImportLegacyRuntime — 阻止新核心包（pkg/agentcore、pkg/coreapi、
//     pkg/protocol/jsonrpc、pkg/sandbox）导入 internal/* 或遗留的 pkg/core facade。
//     新核心包必须通过 coreapi.Engine 接口访问运行时，不得依赖 UI/bridge/runtime 实现细节。
//
//  3. TestToolAPIImplDependencyBoundary — 确保 toolapi/impl 中的 executor.go、tasks.go、
//     bridge.go、services.go 不直接依赖 internal/tools 或 internal/runtime。
//     所有遗留依赖集中在 legacy_bridge.go（legacy adapter）中，便于未来替换。
//
// 维护指南：
//   - 不要重新引入 knownUICoupling 白名单。
//   - 添加新 UI 子包时，确保它不导入 forbiddenUIImports 中的任何包。
//   - 添加新核心包时，确保它不导入 internal/* 或 pkg/core。
//   - TUI 接入 core 只能通过 pkg/coreapi/sidecar/client + coreapi.Engine。
//   - toolapi/impl 中只有 legacy_bridge.go 允许导入 internal/tools 和 internal/runtime。
package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const modulePath = "github.com/dreamSailing/eos"

// knownUICoupling 是历史遗留白名单，目前为空。
// 历史注释：曾允许 internal/ui/adapter 临时 import forbiddenUIImports，
// 迁移完成后已清空，不应再被引用。
var knownUICoupling = map[string]bool{}

var forbiddenUIImports = []string{
	"github.com/dreamSailing/eos/internal/bridge",
	"github.com/dreamSailing/eos/internal/runtime",
	"github.com/dreamSailing/eos/internal/tools",
	"github.com/dreamSailing/eos/pkg/core",
}

func TestUIDirectRuntimeCouplingDoesNotSpread(t *testing.T) {
	root := moduleRoot(t)
	uiRoot := filepath.Join(root, "internal", "ui")
	violations := map[string][]string{}

	err := filepath.WalkDir(uiRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := modulePath + "/" + filepath.ToSlash(rel)
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if isForbiddenUIImport(importPath) && !knownUICoupling[pkg] {
				violations[pkg] = append(violations[pkg], importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk ui imports: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("new UI direct runtime coupling detected: %#v", violations)
	}
}

func TestNewCorePackagesDoNotImportLegacyRuntime(t *testing.T) {
	root := moduleRoot(t)
	packageRoots := []string{
		filepath.Join(root, "pkg", "agentcore"),
		filepath.Join(root, "pkg", "coreapi"),
		filepath.Join(root, "pkg", "protocol", "jsonrpc"),
		filepath.Join(root, "pkg", "sandbox"),
	}
	violations := map[string][]string{}
	for _, packageRoot := range packageRoots {
		err := filepath.WalkDir(packageRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			pkg, err := packagePath(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				if isForbiddenNewCoreImport(importPath) {
					violations[pkg] = append(violations[pkg], importPath)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s imports: %v", packageRoot, err)
		}
	}
	if len(violations) > 0 {
		t.Fatalf("new core package imported legacy runtime/UI: %#v", violations)
	}
}

func isForbiddenUIImport(importPath string) bool {
	for _, forbidden := range forbiddenUIImports {
		if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
			return true
		}
	}
	return false
}

var forbiddenCLIImports = []string{
	"github.com/dreamSailing/eos/internal/bridge",
	"github.com/dreamSailing/eos/internal/tools",
	"github.com/dreamSailing/eos/internal/session",
}

// cliLegacyCoreExceptions 列出了 internal/cli 中允许 import pkg/core (sharedcore) 的文件。
// 当前只有 app_server.go：在 parity mode (--core-engine=parity) 或 EOS_CORE_ALLOW_FALLBACK=1
// 时 legacy 仅作 dev/test fixture。其它生产入口（print / exec / bridge / serve / daemon）一律
// 不允许回退到 sharedcore.Runtime / bridge.RuntimeCore。
var cliLegacyCoreExceptions = map[string]bool{
	"app_server.go": true,
}

// cliForbiddenProductionImports 是 production CLI 命令（非 app_server）必须遵守的禁列。
// 任何新 CLI 命令若想 import sharedcore / bridge.RuntimeCore / eino runtime，
// 必须先把它加入 cliLegacyCoreExceptions 并在注释里说明理由。
var cliForbiddenProductionImports = []string{
	"github.com/dreamSailing/eos/pkg/core",
	"github.com/dreamSailing/eos/internal/bridge",
	"github.com/dreamSailing/eos/internal/runtime",
	"github.com/dreamSailing/eos/internal/tools",
}

func TestCLIHeadlessNoBridgeImport(t *testing.T) {
	root := moduleRoot(t)
	cliRoot := filepath.Join(root, "internal", "cli")
	violations := map[string][]string{}

	err := filepath.WalkDir(cliRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := modulePath + "/" + filepath.ToSlash(rel)
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if isForbiddenCLIImport(importPath) {
				violations[pkg] = append(violations[pkg], importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk cli imports: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("CLI headless direct runtime coupling detected; use pkg/core.Runtime or JSON-RPC client instead:\n%#v", violations)
	}
}

func isForbiddenCLIImport(importPath string) bool {
	for _, forbidden := range forbiddenCLIImports {
		if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
			return true
		}
	}
	return false
}

// TestCLIProductionPathsForbidLegacyGoCore 守护 production CLI 入口（除 app_server）不直接
// import sharedcore（pkg/core）或 internal/bridge / internal/runtime / internal/tools。
//
// app_server 是 parity / dev-only 入口，其默认启动路径走 Rust sidecar；legacy 路径
// 只在 parity 模式或 EOS_CORE_ALLOW_FALLBACK=1 时由 cliLegacyCoreExceptions 放行。
//
// 新 CLI 命令若需要用到 legacy fixture，必须先在 cliLegacyCoreExceptions 加白并写明理由，
// 避免悄悄把 Go core/runtime 重新接回 production 路径。
func TestCLIProductionPathsForbidLegacyGoCore(t *testing.T) {
	root := moduleRoot(t)
	cliRoot := filepath.Join(root, "internal", "cli")
	violations := map[string][]string{}

	err := filepath.WalkDir(cliRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		basename := filepath.Base(path)
		if cliLegacyCoreExceptions[basename] {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		pkg, err := packagePath(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range cliForbiddenProductionImports {
				if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
					violations[pkg+"/"+basename] = append(violations[pkg+"/"+basename], importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk cli imports: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("production CLI path imports legacy Go core (pkg/core / bridge / runtime / tools); use pkg/coreapi/sidecar + engineprovider instead:\n%#v", violations)
	}
}

func isForbiddenNewCoreImport(importPath string) bool {
	if strings.HasPrefix(importPath, modulePath+"/internal/") {
		return true
	}
	return importPath == modulePath+"/pkg/core" || strings.HasPrefix(importPath, modulePath+"/pkg/core/")
}

func packagePath(root, dir string) (string, error) {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return modulePath, nil
	}
	return modulePath + "/" + rel, nil
}

// toolAPIImplLegacyFiles 是 toolapi/impl 中允许导入 internal/tools 和 internal/runtime
// 的遗留适配器文件白名单。只有这些文件可以直接依赖遗留包。
// catalog.go 暂未迁移，待后续重构时从白名单移除。
var toolAPIImplLegacyFiles = map[string]bool{
	"legacy_bridge.go": true,
	"catalog.go":       true,
}

// toolAPIImplForbiddenImports 是 toolapi/impl 中非遗留文件禁止导入的包前缀。
var toolAPIImplForbiddenImports = []string{
	"github.com/dreamSailing/eos/internal/tools",
	"github.com/dreamSailing/eos/internal/runtime",
}

// TestToolAPIImplDependencyBoundary 验证 toolapi/impl 中的 executor.go、tasks.go、
// bridge.go、services.go 不直接依赖 internal/tools 或 internal/runtime。
// 所有遗留依赖应集中在 legacy_bridge.go 中。
func TestToolAPIImplDependencyBoundary(t *testing.T) {
	root := moduleRoot(t)
	implRoot := filepath.Join(root, "internal", "toolapi", "impl")
	violations := map[string][]string{}

	err := filepath.WalkDir(implRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		basename := filepath.Base(path)
		if toolAPIImplLegacyFiles[basename] {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range toolAPIImplForbiddenImports {
				if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
					violations[basename] = append(violations[basename], importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk toolapi/impl imports: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("toolapi/impl clean files have forbidden legacy imports (should be in legacy_bridge.go only):\n%#v", violations)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
