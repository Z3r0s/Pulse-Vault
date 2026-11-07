#!/usr/bin/env python3
"""Measure a real CLI round trip on synthetic files; never use real vaults."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile
import threading
import time

try:
    import psutil
except ImportError:
    psutil = None


def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path, help="path to a built pulse-vault CLI")
    parser.add_argument("--files", type=int, default=100)
    parser.add_argument("--bytes-per-file", type=int, default=65536)
    parser.add_argument("--content", choices=("repeated", "random"), default="repeated",
                        help="repeated compresses well; random approximates already-compressed files")
    parser.add_argument("--profile", choices=("standard", "fast"), default="standard",
                        help="fast is for disposable benchmarks only")
    args = parser.parse_args()
    if args.files < 1 or args.bytes_per_file < 0:
        parser.error("files must be positive and bytes-per-file nonnegative")
    binary = str(args.binary.resolve(strict=True))
    environment = os.environ.copy()
    if args.profile == "fast":
        environment["PULSEVAULT_TEST_FAST_KDF"] = "1"

    with tempfile.TemporaryDirectory(prefix="pulse-vault-benchmark-") as temporary:
        root = Path(temporary)
        source, restored = root / "input", root / "restored"
        source.mkdir()
        expected = {}
        for index in range(args.files):
            relative = Path(f"group-{index % 10:02d}") / f"file-{index:06d}.bin"
            target = source / relative
            target.parent.mkdir(exist_ok=True)
            block = hashlib.sha256(str(index).encode()).digest() * 128
            with target.open("wb") as stream:
                remaining = args.bytes_per_file
                while remaining:
                    size = min(len(block), remaining)
                    chunk = os.urandom(size) if args.content == "random" else block[:size]
                    stream.write(chunk)
                    remaining -= len(chunk)
            expected[str(relative)] = digest(target)

        vault = root / "benchmark.pulsevault"
        timings = {}
        peaks = {}
        def run(name, *command):
            start = time.perf_counter()
            process = subprocess.Popen([binary, *map(str, command), "--password-stdin"],
                                       stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, text=True, env=environment)
            stop = threading.Event()
            samples = []
            def sample_memory():
                if psutil is None:
                    return
                try:
                    observed = psutil.Process(process.pid)
                    while not stop.is_set():
                        samples.append(observed.memory_info().rss)
                        stop.wait(0.02)
                except (psutil.NoSuchProcess, psutil.AccessDenied):
                    pass
            sampler = threading.Thread(target=sample_memory, daemon=True)
            sampler.start()
            stdout, stderr = process.communicate("disposable-benchmark-password\n")
            stop.set()
            sampler.join()
            timings[name] = round(time.perf_counter() - start, 6)
            peaks[name] = max(samples) if samples else None
            if process.returncode:
                raise RuntimeError(f"{name} failed ({process.returncode}): {stderr or stdout}")

        run("create", "create", vault, "--profile", args.profile)
        run("add", "add", vault, source)
        run("verify", "verify", vault)
        run("extract_all", "extract-all", vault, restored)
        actual = {str(path.relative_to(restored)): digest(path)
                  for path in restored.rglob("*") if path.is_file()}
        if actual != expected:
            raise RuntimeError("restored files or SHA-256 hashes do not match inputs")
        print(json.dumps({"platform": platform.platform(), "files": args.files,
                          "input_bytes": args.files * args.bytes_per_file,
                          "vault_bytes": vault.stat().st_size, "profile": args.profile,
                          "content": args.content, "elapsed_seconds": timings,
                          "sampled_peak_rss_bytes": peaks, "verified_files": len(actual)}, indent=2))


if __name__ == "__main__":
    main()
