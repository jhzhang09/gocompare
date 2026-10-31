"""核对受控框架扩展文件的摘要和固定版本，防止依赖更新时静默丢失补丁。"""
import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parents[1]
manifest = json.loads((root / "patches/mygo-v0.7.3-native.json").read_text())
if f"{manifest['module']} {manifest['version']}" not in (root / "go.mod").read_text():
    raise SystemExit("MyGo module version does not match the patch manifest")
for name, entry in manifest["files"].items():
    digest = hashlib.sha256((root / name).read_bytes()).hexdigest()
    if digest != entry["patched"]:
        raise SystemExit(f"MyGo patch checksum mismatch: {name}")
print(f"PASS: {len(manifest['files'])} patched MyGo files verified")
