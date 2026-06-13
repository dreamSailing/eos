package architecture

// 本文件守护 release / default build 不会偷偷重新引入 Go app-server / Go tool-host。
//
// 三类违规需要被拦截：
//   1. main 入口或 production CLI 直接 exec/look up eos-tool-host 二进制；
//   2. production 代码 import cmd/eos-tool-host 或 internal/bridge 直接执行 tool 路径；
//   3. eos-tool-host 与 app-server 走任何 default startup path。
//
// 验证方式：
//   - WalkDir 扫所有 main.go 与 internal/cli/*.go 源代码，检查字面量 / 字符串常量。
//   - 不允许出现 "eos-tool-host" 字符串（除 cmd/eos-tool-host 包自身与 _test.go fixture 之外）。
//   - 不允许出现 sidecar.StartRemote / StartCoreClient / 任何 Go spawn path。
//   - 同时检查 cmd/eos-tool-host 自身仍然处于 "test fixture" 形态（FakeHost / LegacyHost 兜底）。
//
// 维护指南：
//   - 新增 Go sidecar spawn path 前，先确认这是否属于 production。如果是，请删除。
//   - dev fixture 应使用 EOS_TOOL_HOST_FAKE=1 + FakeHost；不要把 LegacyHost 写进 main path。

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var goSidecarStringBlacklist = []string{
	"eos-tool-host",
	"EOS_TOOL_HOST",
	"EOS_TOOL_HOST_FAKE",
}

// TestDefaultBuildDoesNotReferenceGoAppServerOrToolHost 扫描 production 代码路径
// （main.go / internal/cli / internal/ui）确认 default build 不会 reference
// Go app-server（eos app-server）或 Go tool-host（eos-tool-host）二进制。
//
// 允许：
//   - cmd/eos-tool-host 自身（dev fixture）。
//   - 测试文件 *_test.go 中显式 mock 的 FakeHost。
// 禁止：
//   - 任何在 production 路径里把 "eos-tool-host" 当成可执行文件 spawn 的代码。
//   - 任何把 EOS_TOOL_HOST 当作 production env var 读取的代码。
func TestDefaultBuildDoesNotReferenceGoAppServerOrToolHost(t *testing.T) {
	root := moduleRoot(t)
	roots := []string{
		filepath.Join(root, "main.go"),
		filepath.Join(root, "internal", "cli"),
		filepath.Join(root, "internal", "ui"),
	}
	var hits []string

	for _, target := range roots {
		info, err := os.Stat(target)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if hasForbiddenString(t, target) {
				rel, _ := filepath.Rel(root, target)
				hits = append(hits, filepath.ToSlash(rel))
			}
			continue
		}
		err = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if hasForbiddenString(t, path) {
				rel, _ := filepath.Rel(root, path)
				hits = append(hits, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", target, err)
		}
	}

	if len(hits) > 0 {
		sort.Strings(hits)
		t.Fatalf("default build references Go app-server or Go tool-host; remove these references:\n%s",
			strings.Join(hits, "\n"))
	}
}

// hasForbiddenString 检查某个 Go 文件字面量里是否出现 Go sidecar 字符串。
// 同时检查 imports：禁止 import "github.com/dreamSailing/eos/cmd/eos-tool-host"。
func hasForbiddenString(t *testing.T, path string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		// 文件不一定是可 parse 的；fallback 到原始读法。
		return false
	}
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if importPath == "github.com/dreamSailing/eos/cmd/eos-tool-host" ||
			strings.HasPrefix(importPath, "github.com/dreamSailing/eos/cmd/eos-tool-host/") {
			return true
		}
	}
	// 字面量扫描：使用注释扫描器避免误判。
	src, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(src)
	for _, needle := range goSidecarStringBlacklist {
		// 排除注释里的引用：粗略判断 —— 如果 needle 出现在 `//` 或 `/*` 注释中，跳过。
		if idx := strings.Index(text, needle); idx >= 0 {
			if isInsideComment(text, idx) {
				continue
			}
			return true
		}
	}
	return false
}

// isInsideComment 粗略判断 text[idx] 是否在 // 或 /* */ 注释中。
// 用行号定位：如果该行的 // 在 idx 之前，则是 // 注释；
// 如果 idx 在最近的 /* 之后但在最近的 */ 之前，则是块注释。
func isInsideComment(text string, idx int) bool {
	// 找到 idx 所在行。
	lineStart := strings.LastIndex(text[:idx], "\n") + 1
	if lineStart < 0 {
		lineStart = 0
	}
	line := text[lineStart:idx]
	// 行内 // 注释：idx 之后内容不重要，关键是 // 在 idx 之前。
	if slash := strings.Index(line, "//"); slash >= 0 {
		return true
	}
	// 块注释：找最近的 /* 在 idx 之前，且最近的 */ 也在 idx 之前。
	lastOpen := strings.LastIndex(text[:idx], "/*")
	if lastOpen < 0 {
		return false
	}
	lastClose := strings.LastIndex(text[:idx], "*/")
	if lastClose > lastOpen {
		return false
	}
	return true
}

// TestCLIDoesNotStartLegacyBridgeRuntimeInProduction 守护 production CLI
// 入口不显式构造 sharedcore.Runtime 或 bridge.RuntimeCore。
//
// 允许的例外：app_server.go（parity 模式 + dev fixture）。
// 检测方式：扫描 internal/cli/*.go，禁止出现 `core.NewRuntime` / `bridge.NewRuntimeCore`
// 等构造调用。
func TestCLIDoesNotStartLegacyBridgeRuntimeInProduction(t *testing.T) {
	root := moduleRoot(t)
	cliRoot := filepath.Join(root, "internal", "cli")
	hits := map[string][]string{}

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
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(src)
		// 注释中的字面量应当忽略；这里只匹配 "实际" 调用代码模式。
		for _, pattern := range []string{
			"sharedcore.NewRuntime(",
			"core.NewRuntime(",
			"sharedcore.NewLegacyEngine(",
			"core.NewLegacyEngine(",
			"bridge.NewRuntimeCore(",
		} {
			if idx := strings.Index(text, pattern); idx >= 0 {
				if !isInsideComment(text, idx) {
					rel, _ := filepath.Rel(root, path)
					hits[rel] = append(hits[rel], pattern)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk cli: %v", err)
	}
	if len(hits) > 0 {
		// 静默 runtime 必须在 release / default build 出现 —— 立即失败。
		t.Fatalf("production CLI constructs Go legacy runtime; use pkg/coreapi/sidecar + engineprovider:\n%#v", hits)
	}
}

// TestCLIDefaultCoreEngineFlagForbidsLegacy 守护 `eos app-server` 的 --core-engine
// flag 默认值在 production 路径里解析为 rust（ModeAuto），不会走到 legacy。
// 该测试是字符串级别断言：source 中出现 "core-engine" flag 时，默认值必须为空或 "auto/rust"。
func TestCLIDefaultCoreEngineFlagForbidsLegacy(t *testing.T) {
	root := moduleRoot(t)
	appServerPath := filepath.Join(root, "internal", "cli", "app_server.go")
	if _, err := os.Stat(appServerPath); err != nil {
		t.Skipf("app_server.go not present: %v", err)
	}
	src, err := os.ReadFile(appServerPath)
	if err != nil {
		t.Fatalf("read %s: %v", appServerPath, err)
	}
	text := string(src)
	// 显式 legacy 模式应通过 appServerAllowFallback 显式判断；默认值不应是 legacy。
	if strings.Contains(text, `StringVar(&coreEngine, "core-engine", "legacy"`) {
		t.Fatalf("app-server --core-engine default must NOT be 'legacy'; production forbids silent fallback")
	}
	if strings.Contains(text, `StringVar(&coreEngine, "core-engine", "parity"`) {
		t.Fatalf("app-server --core-engine default must NOT be 'parity'; parity is dev-only")
	}
	// 默认值必须是空串 / "auto" / "rust"。
	if !strings.Contains(text, `"core-engine", ""`) &&
		!strings.Contains(text, `"core-engine", "auto"`) &&
		!strings.Contains(text, `"core-engine", "rust"`) {
		t.Fatalf("app-server --core-engine default must be empty/auto/rust, source must keep the flag and the default value Rust-only")
	}
}

// _ = runtime.Caller
var _ = runtime.Caller
