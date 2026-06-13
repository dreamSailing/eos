// 本文件守护 EventBus 接口拆分后的架构边界：
//
// 生产 RemoteEngine（pkg/coreapi/sidecar）只暴露 EventSubscriber，不提供 Publish。
// 生产代码中不允许出现 EventPublisher / EventBus.Publish -> ErrUnsupported 路径。
//
// 规则：
//   - pkg/coreapi/sidecar 下所有非测试 .go 文件不得定义 Publish 方法。
//   - pkg/coreapi/sidecar 下所有非测试 .go 文件不得引用 EventPublisher 或 EventBus 联合接口。
//   - 只有 pkg/core（legacy adapter）允许实现 EventPublisher。
//
// 维护指南：
//   - 如果测试因 "publish method found" 失败：sidecar 中误加了 Publish，请移除。
//   - 如果测试因 "EventBus/EventPublisher reference" 失败：sidecar 中不应依赖发布能力。

package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSidecarRemoteEngineDoesNotExposePublish 确保 pkg/coreapi/sidecar 生产代码
// 不定义任何 Publish 方法（防止 unsupported 路径复活）。
func TestSidecarRemoteEngineDoesNotExposePublish(t *testing.T) {
	root := moduleRoot(t)
	sidecarRoot := filepath.Join(root, "pkg", "coreapi", "sidecar")

	fset := token.NewFileSet()
	var violations []string

	err := filepath.WalkDir(sidecarRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil {
				continue
			}
			if fn.Name.Name == "Publish" {
				violations = append(violations, rel+": method "+fn.Name.Name+" on receiver "+receiverName(fn.Recv))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk sidecar: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("sidecar production code must not define Publish methods (use EventSubscriber-only interface):\n%s",
			strings.Join(violations, "\n"))
	}
}

// TestSidecarProductionDoesNotReferenceEventBusOrPublisher 确保 sidecar 生产代码
// 不引用 EventBus（联合接口）或 EventPublisher 类型。
func TestSidecarProductionDoesNotReferenceEventBusOrPublisher(t *testing.T) {
	root := moduleRoot(t)
	sidecarRoot := filepath.Join(root, "pkg", "coreapi", "sidecar")

	forbidden := []string{"coreapi.EventBus", "coreapi.EventPublisher"}
	var violations []string

	err := filepath.WalkDir(sidecarRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		text := string(src)
		for _, needle := range forbidden {
			if strings.Contains(text, needle) {
				violations = append(violations, rel+" references "+needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk sidecar: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("sidecar production code must not reference EventBus/EventPublisher (use EventSubscriber only):\n%s",
			strings.Join(violations, "\n"))
	}
}

// TestProductionCodeHasNoUnsupportedEventPublish 确保整个生产代码路径中
// 不存在 Events().Publish 调用链（防止调用到 legacy-only 的 Publish）。
func TestProductionCodeHasNoUnsupportedEventPublish(t *testing.T) {
	root := moduleRoot(t)
	forbidden := []string{
		"Events().Publish(",
		"Events().Publish (",
	}
	var violations []string

	productionDirs := []string{
		filepath.Join(root, "internal"),
		filepath.Join(root, "pkg"),
	}
	for _, dir := range productionDirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			text := string(src)
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			for _, needle := range forbidden {
				if strings.Contains(text, needle) {
					violations = append(violations, rel+" contains "+strings.TrimSpace(needle))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if len(violations) > 0 {
		t.Fatalf("production code must not call Events().Publish() (use EventSubscriber.Subscribe only):\n%s",
			strings.Join(violations, "\n"))
	}
}

func receiverName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return "?"
	}
	t := recv.List[0].Type
	switch v := t.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		if id, ok := v.X.(*ast.Ident); ok {
			return "*" + id.Name
		}
	}
	return "?"
}
