package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRawConfig(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSetUpdateProxyEnablesAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos.json")
	if err := setUpdateProxy("zh", "http://127.0.0.1:7897", path); err != nil {
		t.Fatalf("setUpdateProxy: %v", err)
	}

	doc, err := loadRawConfigDoc(path)
	if err != nil {
		t.Fatalf("loadRawConfigDoc: %v", err)
	}
	if !rawConfigDocBool(doc, updateProxyEnabledField) {
		t.Errorf("update_proxy_enabled = false, want true")
	}
	if got := rawConfigDocString(doc, updateProxyURLField); got != "http://127.0.0.1:7897" {
		t.Errorf("update_proxy_url = %q, want http://127.0.0.1:7897", got)
	}
}

func TestSetUpdateProxyOffKeepsURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos.json")
	if err := setUpdateProxy("zh", "http://127.0.0.1:7897", path); err != nil {
		t.Fatalf("setUpdateProxy on: %v", err)
	}
	if err := setUpdateProxy("zh", "off", path); err != nil {
		t.Fatalf("setUpdateProxy off: %v", err)
	}

	doc, err := loadRawConfigDoc(path)
	if err != nil {
		t.Fatalf("loadRawConfigDoc: %v", err)
	}
	if rawConfigDocBool(doc, updateProxyEnabledField) {
		t.Errorf("update_proxy_enabled = true, want false（off 关闭开关）")
	}
	if got := rawConfigDocString(doc, updateProxyURLField); got != "http://127.0.0.1:7897" {
		t.Errorf("update_proxy_url = %q, want 保留原有地址", got)
	}
}

func TestSetUpdateProxyInvalidURLLeavesConfigUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos.json")
	writeRawConfig(t, path, `{"active_model":"MiniMax M3"}`)
	if err := setUpdateProxy("zh", "ftp://127.0.0.1", path); err == nil {
		t.Fatal("setUpdateProxy 应该拒绝非 http/https 地址")
	}

	doc, err := loadRawConfigDoc(path)
	if err != nil {
		t.Fatalf("loadRawConfigDoc: %v", err)
	}
	if rawConfigDocBool(doc, updateProxyEnabledField) {
		t.Errorf("无效地址不应改变 update_proxy_enabled")
	}
	if rawConfigDocString(doc, updateProxyURLField) != "" {
		t.Errorf("无效地址不应写入 update_proxy_url")
	}
	if rawConfigDocString(doc, "active_model") != "MiniMax M3" {
		t.Errorf("无关键被破坏: %v", doc["active_model"])
	}
}

func TestRawConfigDocPreservesUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos.json")
	writeRawConfig(t, path, `{"gui_language":"zh","browser_profiles":[]}`)
	if err := setUpdateProxy("zh", "http://127.0.0.1:7897", path); err != nil {
		t.Fatalf("setUpdateProxy: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"gui_language", "browser_profiles"} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("保存后丢失结构体未声明的键 %q：写入不能走 Config 结构体回写", key)
		}
	}
}
