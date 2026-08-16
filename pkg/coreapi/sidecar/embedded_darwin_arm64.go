//go:build darwin && arm64

package sidecar

import _ "embed"

//go:embed core/aarch64-apple-darwin/eos-core
var embeddedCoreBinary []byte

//go:embed core/aarch64-apple-darwin/manifest.json
var embeddedCoreManifestBytes []byte

func init() {
	embeddedCoreSidecar = func() ([]byte, []byte, bool) {
		return embeddedCoreBinary, embeddedCoreManifestBytes, true
	}
}
