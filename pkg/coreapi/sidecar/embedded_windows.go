//go:build windows

package sidecar

import _ "embed"

// vendored 内核（dev-rebuild / sync workflow 产出的签名产物），仅内嵌当前
// 平台的一份，避免二进制体积随平台数膨胀。
//
//go:embed core/x86_64-pc-windows-gnu/eos-core.exe
var embeddedCoreBinary []byte

//go:embed core/x86_64-pc-windows-gnu/manifest.json
var embeddedCoreManifestBytes []byte

func init() {
	embeddedCoreSidecar = func() ([]byte, []byte, bool) {
		return embeddedCoreBinary, embeddedCoreManifestBytes, true
	}
}
