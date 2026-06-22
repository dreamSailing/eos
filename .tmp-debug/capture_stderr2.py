"""Capture eos-core stderr while making a turn/start request — full init flow."""
import subprocess, json, sys, os, time, threading

CORE = r"C:\home\eos\eos-cli\pkg\coreapi\sidecar\binaries\x86_64-pc-windows-gnu\eos-core.exe"

proc = subprocess.Popen(
    [CORE, "--stdio"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
)

next_id = [0]
def next_req_id():
    next_id[0] += 1
    return next_id[0]

def rpc(method, params=None):
    rid = next_req_id()
    msg = {"jsonrpc": "2.0", "method": method, "params": params or {}, "id": rid}
    payload = json.dumps(msg, ensure_ascii=False) + "\n"
    proc.stdin.write(payload.encode())
    proc.stdin.flush()
    line = proc.stdout.readline().decode('utf-8', errors='replace')
    try:
        return json.loads(line)
    except:
        return {"raw": line}

def read_stderr():
    while True:
        line = proc.stderr.readline()
        if not line:
            break
        text = line.decode('utf-8', errors='replace').rstrip()
        if text:
            print(f"[STDERR] {text}", flush=True)

stderr_thread = threading.Thread(target=read_stderr, daemon=True)
stderr_thread.start()

# 1. Initialize
print(">>> initialize")
resp = rpc("initialize", {"client_name": "debug", "client_version": "1.0"})
print(f"<<< {resp.get('id','?')}: error={resp.get('error','ok')}")

# 2. Workspace
print(">>> workspace/remember")
resp = rpc("workspace/remember", {"root": os.getcwd()})
print(f"<<< {resp.get('id','?')}: error={resp.get('error','ok')}")

# 3. Model config — use env vars for MiniMax
model_name = os.environ.get("EOS_TEST_MODEL", "MiniMax-M3")
print(f">>> model/upsert (using env EOS_API_KEY, model={model_name})")
resp = rpc("model/upsert", {
    "name": "test-model",
    "api_base": "https://api.minimaxi.com/v1",
    "api_key_env": "EOS_API_KEY",
    "model": model_name,
    "provider": "openai_compatible",
})
print(f"<<< {resp.get('id','?')}: error={resp.get('error','ok')}")

print(">>> model/activate")
resp = rpc("model/activate", {"model_name": "test-model"})
print(f"<<< {resp.get('id','?')}: error={resp.get('error','ok')}")

# 4. Session
print(">>> session/create")
resp = rpc("session/create", {"workspace_root": os.getcwd()})
print(f"<<< {resp.get('id','?')}: result_keys={list(resp.get('result',{}).keys())}")
session_id = resp.get("result", {}).get("id", "")

# 5. Event subscription
print(">>> event/subscribe")
rpc("event/subscribe", {"event_types": ["turn.started", "turn.completed", "turn.error", "turn.item_started", "turn.item_delta", "turn.item_completed"]})

# 6. Turn start
print(">>> turn/start")
resp = rpc("turn/start", {
    "session_id": session_id,
    "input": "当前是什么项目？请列出当前目录的文件。",
})
print(f"<<< {resp.get('id','?')}: error={resp.get('error','ok')}, result_keys={list(resp.get('result',{}).keys())}")

# 7. Read events for a while
print(">>> Reading events...")
deadline = time.time() + 30
while time.time() < deadline:
    line = proc.stdout.readline().decode('utf-8', errors='replace')
    if not line:
        if proc.poll() is not None:
            print(">>> Process exited")
            break
        time.sleep(0.1)
        continue
    try:
        msg = json.loads(line)
        method = msg.get("method", "")
        if method in ("turn.completed", "turn.error"):
            params = msg.get("params", {})
            print(f"[EVENT] {method} → {json.dumps(params, ensure_ascii=False)[:500]}")
            break
        elif method:
            print(f"[EVENT] {method}")
        else:
            print(f"[RESP] id={msg.get('id','?')}")
    except:
        print(f"[RAW] {line[:200]}")

time.sleep(1)
proc.terminate()
print("Done")
