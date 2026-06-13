//go:build legacy

package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/coreapi"
)

type EngineKind string

const (
	EngineLegacy  EngineKind = "legacy"
	EngineSidecar EngineKind = "sidecar"
)

type EngineSetup struct {
	Kind   EngineKind
	Engine coreapi.Engine
	Close  func()
}

type ParityHarness struct {
	t          *testing.T
	engines    []EngineSetup
	gaps       []ExpectedGap
	normalizer *Normalizer
}

func NewParityHarness(t *testing.T, engines ...EngineSetup) *ParityHarness {
	t.Helper()
	if len(engines) < 2 {
		t.Fatal("parity harness requires at least 2 engines")
	}
	return &ParityHarness{
		t:          t,
		engines:    engines,
		normalizer: NewNormalizer(),
		gaps:       LoadExpectedGaps(),
	}
}

func (h *ParityHarness) Close() {
	for _, e := range h.engines {
		if e.Close != nil {
			e.Close()
		}
	}
}

type OperationResult struct {
	Engine EngineKind
	Label  string
	Data   any
	Err    error
}

func (h *ParityHarness) RunOperation(name string, fn func(coreapi.Engine) (any, error)) []OperationResult {
	h.t.Helper()
	results := make([]OperationResult, len(h.engines))
	for i, e := range h.engines {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = ctx
		data, err := fn(e.Engine)
		cancel()
		results[i] = OperationResult{
			Engine: e.Kind,
			Label:  name,
			Data:   data,
			Err:    err,
		}
		h.t.Logf("parity[%s/%s]: err=%v", name, e.Kind, errToString(err))
	}
	return results
}

func (h *ParityHarness) CompareResults(name string, results []OperationResult) {
	h.t.Helper()

	legacy := results[0]
	sidecar := results[1]

	if legacy.Err != nil && sidecar.Err != nil {
		legacyUnsupported := isUnsupported(legacy.Err)
		sidecarUnsupported := isUnsupported(sidecar.Err)
		if legacyUnsupported != sidecarUnsupported {
			h.t.Errorf("parity[%s]: unsupported mismatch: legacy=%v sidecar=%v",
				name, legacyUnsupported, sidecarUnsupported)
		}
		return
	}

	if legacy.Err != nil || sidecar.Err != nil {
		if h.isExpectedGap(name, "error_presence") {
			h.t.Logf("parity[%s]: EXPECTED GAP - error presence differs: legacy_err=%v sidecar_err=%v",
				name, legacy.Err != nil, sidecar.Err != nil)
			return
		}
		h.t.Errorf("parity[%s]: error presence differs: legacy_err=%v sidecar_err=%v\n  legacy: %v\n  sidecar: %v",
			name, legacy.Err != nil, sidecar.Err != nil, legacy.Err, sidecar.Err)
		return
	}

	legacyNorm := h.normalizer.Normalize(legacy.Data)
	sidecarNorm := h.normalizer.Normalize(sidecar.Data)

	diffs := collectDiffs(legacyNorm, sidecarNorm, "$")
	if len(diffs) == 0 {
		return
	}

	var unexpected []string
	for _, d := range diffs {
		if h.isExpectedGap(name, d.Path) {
			h.t.Logf("parity[%s]: EXPECTED GAP - %s", name, d.String())
			continue
		}
		unexpected = append(unexpected, d.String())
	}

	if len(unexpected) > 0 {
		legacyJSON, _ := json.MarshalIndent(legacyNorm, "", "  ")
		sidecarJSON, _ := json.MarshalIndent(sidecarNorm, "", "  ")
		h.t.Errorf("parity[%s]: %d unexpected difference(s):\n%s\n\n  legacy normalized:\n%s\n\n  sidecar normalized:\n%s",
			name, len(unexpected), strings.Join(unexpected, "\n"), string(legacyJSON), string(sidecarJSON))
	}
}

func (h *ParityHarness) isExpectedGap(operation, fieldPath string) bool {
	for _, g := range h.gaps {
		if g.Operation != operation {
			continue
		}
		if g.Field == "*" || strings.Contains(fieldPath, g.Field) {
			return true
		}
	}
	return false
}

type DiffEntry struct {
	Path   string
	Legacy any
	Sidecar any
}

func (d DiffEntry) String() string {
	return fmt.Sprintf("%s:\n    legacy:  %v\n    sidecar: %v", d.Path, d.Legacy, d.Sidecar)
}

func collectDiffs(a, b any, path string) []DiffEntry {
	if a == nil && b == nil {
		return nil
	}
	if a == nil || b == nil {
		return []DiffEntry{{Path: path, Legacy: a, Sidecar: b}}
	}

	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	var aNorm, bNorm any
	json.Unmarshal(aJSON, &aNorm)
	json.Unmarshal(bJSON, &bNorm)

	return deepCollectDiffs(aNorm, bNorm, path)
}

func deepCollectDiffs(a, b any, path string) []DiffEntry {
	if a == nil && b == nil {
		return nil
	}
	if a == nil || b == nil {
		return []DiffEntry{{Path: path, Legacy: a, Sidecar: b}}
	}

	aMap, aIsMap := a.(map[string]any)
	bMap, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		return diffMaps(aMap, bMap, path)
	}

	aSlice, aIsSlice := a.([]any)
	bSlice, bIsSlice := b.([]any)
	if aIsSlice && bIsSlice {
		return diffSlices(aSlice, bSlice, path)
	}

	if !reflect.DeepEqual(a, b) {
		return []DiffEntry{{Path: path, Legacy: a, Sidecar: b}}
	}
	return nil
}

func diffMaps(a, b map[string]any, path string) []DiffEntry {
	var diffs []DiffEntry
	allKeys := make(map[string]bool)
	for k := range a {
		allKeys[k] = true
	}
	for k := range b {
		allKeys[k] = true
	}

	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		childPath := fmt.Sprintf("%s.%s", path, k)
		aVal, aOk := a[k]
		bVal, bOk := b[k]

		if !aOk {
			diffs = append(diffs, DiffEntry{Path: childPath, Legacy: nil, Sidecar: bVal})
			continue
		}
		if !bOk {
			diffs = append(diffs, DiffEntry{Path: childPath, Legacy: aVal, Sidecar: nil})
			continue
		}

		diffs = append(diffs, deepCollectDiffs(aVal, bVal, childPath)...)
	}
	return diffs
}

func diffSlices(a, b []any, path string) []DiffEntry {
	if len(a) != len(b) {
		return []DiffEntry{{
			Path:    path + ".length",
			Legacy:  len(a),
			Sidecar: len(b),
		}}
	}

	var diffs []DiffEntry
	for i := range a {
		childPath := fmt.Sprintf("%s[%d]", path, i)
		diffs = append(diffs, deepCollectDiffs(a[i], b[i], childPath)...)
	}
	return diffs
}

type Normalizer struct {
	ignoredFields map[string]bool
	idFields      map[string]bool
}

func NewNormalizer() *Normalizer {
	return &Normalizer{
		ignoredFields: map[string]bool{
			"timestamp":    true,
			"updated_at":   true,
			"created_at":   true,
			"started_at":   true,
			"ended_at":     true,
			"finished_at":  true,
			"last_used_at": true,
			"duration":     true,
		},
		idFields: map[string]bool{
			"id":               true,
			"session_id":       true,
			"turn_id":          true,
			"thread_id":        true,
			"request_id":       true,
			"correlation_id":   true,
			"event_id":         true,
			"approval_id":      true,
			"inquiry_id":       true,
			"task_id":          true,
			"agent_id":         true,
			"parent_agent_id":  true,
			"from_agent_id":    true,
			"subscription_id":  true,
			"current_session_id": true,
		},
	}
}

func (n *Normalizer) Normalize(data any) any {
	if data == nil {
		return nil
	}
	v := n.normalizeValue(reflect.ValueOf(data))
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

func (n *Normalizer) normalizeValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return reflect.ValueOf(nil)
	}

	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		return n.normalizeValue(v.Elem())
	case reflect.Ptr:
		if v.IsNil() {
			return v
		}
		elem := n.normalizeValue(v.Elem())
		ptr := reflect.New(elem.Type())
		ptr.Elem().Set(elem)
		return ptr
	case reflect.Struct:
		return n.normalizeStruct(v)
	case reflect.Map:
		return n.normalizeMap(v)
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		result := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			result.Index(i).Set(n.normalizeValue(v.Index(i)))
		}
		return result
	default:
		return v
	}
}

func (n *Normalizer) normalizeStruct(v reflect.Value) reflect.Value {
	t := v.Type()
	result := reflect.New(t).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldVal := v.Field(i)
		jsonTag := field.Tag.Get("json")
		jsonName := strings.Split(jsonTag, ",")[0]
		if jsonName == "" {
			jsonName = field.Name
		}

		lowerName := strings.ToLower(jsonName)
		if n.ignoredFields[lowerName] {
			continue
		}
		if n.idFields[lowerName] {
			if fieldVal.Kind() == reflect.String && fieldVal.String() != "" {
				result.Field(i).Set(reflect.ValueOf("<id>"))
			}
			continue
		}

		result.Field(i).Set(n.normalizeValue(fieldVal))
	}
	return result
}

func (n *Normalizer) normalizeMap(v reflect.Value) reflect.Value {
	if v.IsNil() {
		return v
	}
	result := reflect.MakeMap(v.Type())
	for _, key := range v.MapKeys() {
		keyStr := key.String()
		lowerKey := strings.ToLower(keyStr)

		if n.ignoredFields[lowerKey] {
			continue
		}
		if n.idFields[lowerKey] {
			val := v.MapIndex(key)
			if val.Kind() == reflect.Interface {
				val = val.Elem()
			}
			if val.Kind() == reflect.String && val.String() != "" {
				result.SetMapIndex(key, reflect.ValueOf("<id>"))
			} else {
				result.SetMapIndex(key, n.normalizeValue(v.MapIndex(key)))
			}
			continue
		}

		result.SetMapIndex(key, n.normalizeValue(v.MapIndex(key)))
	}
	return result
}

func isUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unsupported")
}

func errToString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
