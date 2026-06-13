package sidecar

import (
	"strings"
	"testing"
)

func TestMergedEnvOverridesExistingKeysCaseInsensitively(t *testing.T) {
	base := []string{
		"EOS_CORE_STORE_DIR=C:/old-store",
		"EOS_MODEL_PROVIDER=fake",
	}
	extra := map[string]string{
		"eos_core_store_dir": "C:/new-store",
		"EOS_MODEL_PROVIDER": "",
	}

	got := mergedEnv(base, extra)
	seen := map[string]string{}
	for _, item := range got {
		key, value, ok := splitEnvKV(item)
		if !ok {
			continue
		}
		seen[normalizeEnvKey(key)] = value
	}

	if seen["EOS_CORE_STORE_DIR"] != "C:/new-store" {
		t.Fatalf("EOS_CORE_STORE_DIR=%q, want C:/new-store", seen["EOS_CORE_STORE_DIR"])
	}
	if seen["EOS_MODEL_PROVIDER"] != "" {
		t.Fatalf("EOS_MODEL_PROVIDER=%q, want empty override", seen["EOS_MODEL_PROVIDER"])
	}
	if countEnvKey(got, "EOS_CORE_STORE_DIR") != 1 {
		t.Fatalf("EOS_CORE_STORE_DIR appears %d times, want 1", countEnvKey(got, "EOS_CORE_STORE_DIR"))
	}
	if countEnvKey(got, "EOS_MODEL_PROVIDER") != 1 {
		t.Fatalf("EOS_MODEL_PROVIDER appears %d times, want 1", countEnvKey(got, "EOS_MODEL_PROVIDER"))
	}
}

func splitEnvKV(item string) (string, string, bool) {
	for i := 0; i < len(item); i++ {
		if item[i] == '=' {
			return item[:i], item[i+1:], true
		}
	}
	return "", "", false
}

func countEnvKey(items []string, key string) int {
	count := 0
	for _, item := range items {
		k, _, ok := splitEnvKV(item)
		if ok && normalizeEnvKey(k) == normalizeEnvKey(key) {
			count++
		}
	}
	return count
}

func normalizeEnvKey(key string) string {
	return strings.ToUpper(key)
}
