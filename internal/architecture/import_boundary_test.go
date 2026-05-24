// Package architecture 守护 EOS 架构边界，防止迁移过程中耦合回退。
//
// 本文件定义了三层导入边界守卫：
//
//  1. TestUIDirectRuntimeCouplingDoesNotSpread — 阻止 internal/ui 下除 adapter 外的所有包
//     导入 internal/bridge、internal/runtime、internal/tools。adapter 是唯一的兼容性孤岛，
//     仅允许导入 internal/bridge（用于 legacy 事件归一化和 session 类型）。
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
//   - 不要扩大 knownUICoupling，除非有明确的迁移计划和文档说明。
//   - 添加新 UI 子包时，确保它不导入 forbiddenUIImports 中的任何包。
//   - 添加新核心包时，确保它不导入 internal/* 或 pkg/core。
//   - 当 adapter 完成 bridge/runtime/tools 解耦后，从 knownUICoupling 中移除它。
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

// knownUICoupling 是当前允许导入 forbiddenUIImports 的 UI 包白名单。
// 仅 internal/ui/adapter 作为兼容性孤岛被允许，待其完成 bridge 解耦后应移除。
var knownUICoupling = map[string]bool{
	"github.com/dreamSailing/eos/internal/ui/adapter": true,
}

var forbiddenUIImports = []string{
	"github.com/dreamSailing/eos/internal/bridge",
	"github.com/dreamSailing/eos/internal/runtime",
	"github.com/dreamSailing/eos/internal/tools",
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
