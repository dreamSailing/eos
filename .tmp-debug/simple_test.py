"""Simple core init test."""
import subprocess, json, os

CORE = r"C:\home\eos\eos-cli\pkg\coreapi\sidecar\binaries\x86_64-pc-windows-gnu\eos-core.exe"

proc = subprocess.Popen(
    [CORE, "--stdio"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
)

def rpc(method, params=None):
    msg = json.dumps({"jsonrpc":"2.0","method":method,"params":params or {},"id":1}) + "\n"
    proc.stdin.write(msg.encode())
    proc.stdin.flush()
    line = proc.stdout.readline().decode('utf-8', errors='replace')
    print(f"{method} → {line[:300]}")
    return line

rpc("initialize", {"client_name":"t","client_version":"1"})
rpc("workspace/remember", {"root": os.getcwd()})
rpc("session/create", {"workspace_root": os.getcwd()})
rpc("turn/start", {"session_id":"test","input":"hello"})

proc.terminate()
