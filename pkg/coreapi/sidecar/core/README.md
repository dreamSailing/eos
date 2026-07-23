# Closed core binaries

Vendored release artifacts for the private Rust core belong in:

```text
pkg/coreapi/sidecar/binaries/<target>/manifest.json
pkg/coreapi/sidecar/binaries/<target>/eos-core[.exe]
```

Do not place Rust source, Cargo metadata, debug symbols, private keys, or unstripped artifacts in this directory.
