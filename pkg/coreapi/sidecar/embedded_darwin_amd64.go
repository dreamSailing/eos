//go:build darwin && amd64 && !eos_noembed

package sidecar

import _ "embed"

//go:embed core/x86_64-apple-darwin/eos-core
var embeddedCoreBinary []byte

//go:embed core/x86_64-apple-darwin/manifest.json
var embeddedCoreManifestBytes []byte

func init() {
	embeddedCoreSidecar = func() ([]byte, []byte, bool) {
		return embeddedCoreBinary, embeddedCoreManifestBytes, true
	}
}
