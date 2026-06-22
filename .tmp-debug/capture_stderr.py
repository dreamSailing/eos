"""Capture eos-core stderr while making a turn/start request."""
import subprocess, json, sys, os, time, threading

CORE = r"C:\home\eos\eos-cli\pkg\coreapi\sidecar\binaries\x86_64-pc-windows-gnu\eos-core.exe"

def read_stderr(proc, lines):
    for line in proc.stderr:
        line = line.decode('utf-8', errors='replace').rstrip()
        lines.append(line)
        print(f"[STDERR] {line}", flush=True)

proc = subprocess.Popen(
    [CORE, "--stdio"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
)

stderr_lines = []
t = threading.Thread(target=read_stderr, args=(proc, stderr_lines), daemon=True)
t.start()

def rpc(method, params=None, request_id=1):
    msg = {"jsonrpc": "2.0", "method": method, "params": params or {}, "id": request_id}
    payload = json.dumps(msg) + "\n"
    proc.stdin.write(payload.encode())
    proc.stdin.flush()
    # Read response
    line = proc.stdout.readline().decode('utf-8', errors='replace')
    try:
        return json.loads(line)
    except:
        return line

# 1. Initialize
print(">>> initialize")
resp = rpc("initialize", {"client_name": "debug", "client_version": "1.0"})
print(f"<<< {json.dumps(resp, indent=2)[:500]}")

# 2. Create session
print(">>> session/create")
resp = rpc("session/create", {
    "workspace_root": os.getcwd(),
}, request_id=2)
print(f"<<< {json.dumps(resp, indent=2)[:500]}")
session_id = resp.get("result", {}).get("session_id", "")

# 3. Turn start
print(">>> turn/start")
resp = rpc("turn/start", {
    "session_id": session_id,
    "input": "你好，当前是什么项目？",
}, request_id=3)
print(f"<<< {json.dumps(resp, indent=2)[:500]}")

# Wait a bit for stderr to flush
time.sleep(3)

# Read events
print(">>> Reading events from stdout...")
proc.stdout.flush()
try:
    while True:
        line = proc.stdout.readline().decode('utf-8', errors='replace')
        if not line:
            break
        try:
            msg = json.loads(line)
            if msg.get("method") in ("turn.completed", "turn.error"):
                print(f"[EVENT] {msg.get('method')} → {json.dumps(msg.get('params',{}))[:300]}")
                break
            print(f"[EVENT] {msg.get('method', '?')}")
        except:
            pass
except:
    pass

time.sleep(1)
proc.terminate()
print(f"\n=== Captured {len(stderr_lines)} stderr lines ===")
for l in stderr_lines:
    print(f"  {l}")
