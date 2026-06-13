package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/dreamSailing/eos/pkg/coreapi/sidecar"
)

func main() {
    opts := sidecar.ProcessOptions{
        Env: map[string]string{
            "EOS_CORE_STORE_DIR": os.Getenv("EOS_CORE_STORE_DIR"),
        },
        VerifyChecksum: true,
        RequireSignature: true,
    }
    resolved, err := sidecar.ResolveBinary(sidecar.ResolveOptions{
        VerifyChecksum: true,
        RequireSignature: true,
    })
    fmt.Printf("resolve: binary=%q manifest=%q target=%q source=%q err=%v\n", resolved.Path, resolved.ManifestPath, resolved.Target, resolved.Source, err)
    if err != nil {
        return
    }
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    eng, err := sidecar.StartRemoteEngine(ctx, opts)
    fmt.Printf("start: err=%v\n", err)
    if err != nil {
        return
    }
    defer eng.Close()
    init, err := eng.Initialize(ctx)
    fmt.Printf("initialize: methods=%d err=%v\n", len(init.Methods), err)
}
