//go:build linux && arm64 && !eos_noembed

package sidecar

import _ "embed"

//go:embed core/aarch64-unknown-linux-gnu/eos-core
var embeddedCoreBinary []byte

//go:embed core/aarch64-unknown-linux-gnu/manifest.json
var embeddedCoreManifestBytes []byte

func init() {
	embeddedCoreSidecar = func() ([]byte, []byte, bool) {
		return embeddedCoreBinary, embeddedCoreManifestBytes, true
	}
}
