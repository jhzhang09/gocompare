"""通过 MyGo e2e 接口验证真实窗口的双侧输入、撤销和焦点侧保存。"""
import argparse
import base64
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--directories", action="store_true")
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="gocompare-smoke-") as directory:
        root = Path(directory)
        paths = [root / "left.txt", root / "right.txt"]
        for path, text in zip(paths, ["header\nleft\n", "header\nright\n"]):
            path.write_text(text, encoding="utf-8")
        launch_paths = paths
        if args.directories:
            launch_paths = [root / "left-dir", root / "right-dir"]
            for directory, source in zip(launch_paths, paths):
                directory.mkdir()
                (directory / "different.txt").write_bytes(source.read_bytes())
            paths = [directory / "different.txt" for directory in launch_paths]
        endpoint = str(root / "automation.sock")
        with (args.output / "native.log").open("w") as log:
            process = subprocess.Popen(
                [str(args.binary.resolve()), *(["--directories"] if args.directories else []), *map(str, launch_paths)],
                env=dict(os.environ, MYGO_AUTOMATION=endpoint, GOCOMPARE_DATA_DIR=str(root / "data")),
                stdout=log, stderr=log,
            )
            connection = None
            try:
                deadline = time.monotonic() + 10
                while not Path(endpoint).exists():
                    if process.poll() is not None or time.monotonic() > deadline:
                        raise RuntimeError("native application did not start")
                    time.sleep(0.05)
                connection = socket.socket(socket.AF_UNIX)
                connection.settimeout(10)
                connection.connect(endpoint)
                with connection.makefile("rwb") as stream:
                    request_id = 0

                    def call(method, **params):
                        nonlocal request_id
                        request_id += 1
                        stream.write((json.dumps(dict(id=request_id, method=method, params=params)) + "\n").encode())
                        stream.flush()
                        response = json.loads(stream.readline())
                        if "error" in response:
                            raise RuntimeError(response["error"])
                        return response["result"]

                    def nodes():
                        return call("tree")["nodes"]

                    def field(label):
                        return next(node for node in nodes() if node.get("label") == label)

                    deadline = time.monotonic() + 10
                    while not call("windows")["windows"]:
                        if time.monotonic() > deadline:
                            raise RuntimeError("native window was not created")
                        time.sleep(0.05)
                    if args.directories:
                        while not any(node.get("label") == "different.txt" for node in nodes()):
                            if time.monotonic() > deadline:
                                raise RuntimeError("directory comparison did not load")
                            time.sleep(0.05)
                        node = next(node for node in nodes() if node.get("label") == "different.txt")
                        call("act", id=node["id"], action="press")
                    while not any(node.get("label") == "Old text" for node in nodes()):
                        if time.monotonic() > deadline:
                            raise RuntimeError("comparison did not load")
                        time.sleep(0.05)
                    for side, label in enumerate(["Old text", "New text"]):
                        node = field(label)
                        assert "readOnly" not in node["states"], node
                        before = node["value"]
                        call("act", id=node["id"], action="focus")
                        text = f"第 {side + 1} 侧修改\n\n末行"
                        call("input", events=[dict(type="key", key="CmdOrCtrl+A"), dict(type="text", text=text)])
                        assert field(label)["value"] == text
                        call("input", events=[dict(type="key", key="CmdOrCtrl+Z")])
                        assert field(label)["value"] == before
                        call("input", events=[dict(type="key", key="CmdOrCtrl+A"), dict(type="text", text=text), dict(type="key", key="CmdOrCtrl+S")])
                        deadline = time.monotonic() + 10
                        while paths[side].read_text(encoding="utf-8") != text:
                            if time.monotonic() > deadline:
                                raise RuntimeError("focused side was not saved")
                            time.sleep(0.05)
                    if args.directories:
                        node = next(node for node in nodes() if node.get("label") == "返回目录")
                        call("act", id=node["id"], action="press")
                        deadline = time.monotonic() + 10
                        while not any(node.get("label") == "different.txt" for node in nodes()):
                            if time.monotonic() > deadline:
                                raise RuntimeError("directory navigation did not return")
                            time.sleep(0.05)
                    result = call("capture")
                    (args.output / "native.png").write_bytes(base64.b64decode(result["png"]))
                    (args.output / "tree.json").write_text(json.dumps(call("tree"), ensure_ascii=False, indent=2), encoding="utf-8")
                    print("PASS: native dual edit, undo, focused save and capture")
            finally:
                if connection is not None:
                    connection.close()
                if process.poll() is None:
                    process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    main()
