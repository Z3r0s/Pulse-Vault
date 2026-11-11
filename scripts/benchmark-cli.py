#!/usr/bin/env python3
"""Measure real CLI round trips on synthetic files; never use real vaults."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import statistics
import subprocess
import sys
import tempfile
import threading
import time

try:
    import psutil
except ImportError:
    psutil = None


def rss_bytes_from_statm(contents, page_size):
    """Return resident bytes from Linux /proc/<pid>/statm contents."""
    fields = contents.split()
    if len(fields) < 2:
        return None
    try:
        return int(fields[1]) * page_size
    except ValueError:
        return None


def linux_process_rss_bytes(pid, proc_root=Path("/proc"), page_size=None):
    try:
        if page_size is None:
            page_size = os.sysconf("SC_PAGE_SIZE")
        with (Path(proc_root) / str(pid) / "statm").open(encoding="ascii") as statm:
            return rss_bytes_from_statm(statm.read(), page_size)
    except (OSError, ValueError):
        return None


def process_rss_bytes(pid):
    if psutil is not None:
        try:
            return psutil.Process(pid).memory_info().rss
        except (psutil.NoSuchProcess, psutil.AccessDenied):
            pass

    if sys.platform.startswith("linux"):
        return linux_process_rss_bytes(pid)
    return None


def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def run_once(binary, args, environment):
    with tempfile.TemporaryDirectory(prefix="pulse-vault-benchmark-") as temporary:
        root = Path(temporary)
        source = root / "input-initial"
        incremental = root / "input-incremental"
        restored = root / "restored"
        source.mkdir()
        incremental.mkdir()
        expected = {}
        initial_names = []
        split_at = max(1, args.files // 2)
        for index in range(args.files):
            relative = Path(f"group-{index % 10:02d}") / f"file-{index:06d}.bin"
            batch = source if index < split_at else incremental
            target = batch / relative
            target.parent.mkdir(exist_ok=True)
            block = hashlib.sha256(str(index).encode()).digest() * 128
            with target.open("wb") as stream:
                remaining = args.bytes_per_file
                while remaining:
                    size = min(len(block), remaining)
                    chunk = os.urandom(size) if args.content == "random" else block[:size]
                    stream.write(chunk)
                    remaining -= size
            expected[relative.as_posix()] = digest(target)
            if index < split_at:
                initial_names.append(relative.as_posix())

        vault = root / "benchmark.pulsevault"
        timings = {}
        peaks = {}

        def run(name, *command):
            start = time.perf_counter()
            process = subprocess.Popen(
                [binary, *map(str, command), "--password-stdin"],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                env=environment,
            )
            stop = threading.Event()
            samples = []

            def sample_memory():
                while not stop.is_set():
                    sampled_rss = process_rss_bytes(process.pid)
                    if sampled_rss is not None:
                        samples.append(sampled_rss)
                    stop.wait(0.02)

            initial_sample = process_rss_bytes(process.pid)
            if initial_sample is not None:
                samples.append(initial_sample)
            sampler = threading.Thread(target=sample_memory, daemon=True)
            sampler.start()
            stdout, stderr = process.communicate("disposable-benchmark-password\n")
            stop.set()
            sampler.join()
            if args.require_memory_sampling and not samples:
                raise RuntimeError(f"{name} produced no process RSS samples")
            timings[name] = round(time.perf_counter() - start, 6)
            peaks[name] = max(samples) if samples else None
            if process.returncode:
                raise RuntimeError(f"{name} failed ({process.returncode}): {stderr or stdout}")
            return stdout

        run("create", "create", vault, "--profile", args.profile)
        run("add_initial", "add", vault, source)
        if args.files > split_at:
            # Add new names to a populated vault to exercise the incremental
            # conflict-index path as well as bulk IO.
            run("add_incremental", "add", vault, incremental)
        verify_output = run("verify", "verify", vault)
        if f"Verify OK: {args.files} files," not in verify_output:
            raise RuntimeError(f"verify reported an unexpected file count: {verify_output}")
        run("extract_all", "extract-all", vault, restored)
        actual = {
            path.relative_to(restored).as_posix(): digest(path)
            for path in restored.rglob("*")
            if path.is_file()
        }
        if actual != expected:
            raise RuntimeError("restored files or SHA-256 hashes do not match inputs")

        vault_bytes_before_delete = vault.stat().st_size
        delete_count = min(max(1, split_at), 100)
        delete_names = initial_names[:delete_count]
        run("delete_batch", "delete", vault, *delete_names)
        remaining_count = args.files - delete_count
        verify_after_delete = run("verify_after_delete", "verify", vault)
        if f"Verify OK: {remaining_count} files," not in verify_after_delete:
            raise RuntimeError(
                f"post-delete verify reported an unexpected file count: {verify_after_delete}"
            )

        return {
            "vault_bytes": vault_bytes_before_delete,
            "vault_bytes_after_delete": vault.stat().st_size,
            "deleted_files": delete_count,
            "remaining_files": remaining_count,
            "elapsed_seconds": timings,
            "sampled_peak_rss_bytes": peaks,
            "verified_files": len(actual),
        }


def median_for_stage(run_samples, field, stage, round_result=True):
    values = [sample[field][stage] for sample in run_samples if sample[field][stage] is not None]
    if not values:
        return None
    value = statistics.median(values)
    return round(value, 6) if round_result else int(round(value))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path, help="path to a built pulse-vault CLI")
    parser.add_argument("--files", type=int, default=100)
    parser.add_argument("--bytes-per-file", type=int, default=65536)
    parser.add_argument(
        "--content",
        choices=("repeated", "random"),
        default="repeated",
        help="repeated compresses well; random approximates already-compressed files",
    )
    parser.add_argument(
        "--profile",
        choices=("standard", "fast"),
        default="standard",
        help="fast is for disposable tests only",
    )
    parser.add_argument(
        "--runs",
        type=int,
        default=1,
        help="repeat the complete workload and report median timings and RSS",
    )
    parser.add_argument(
        "--require-memory-sampling",
        action="store_true",
        help="fail a run if a CLI operation has no process RSS samples",
    )
    args = parser.parse_args()
    if args.files < 1 or args.bytes_per_file < 0 or args.runs < 1:
        parser.error("files and runs must be positive; bytes-per-file must be nonnegative")
    binary = str(args.binary.resolve(strict=True))
    environment = os.environ.copy()
    if args.profile == "fast":
        environment["PULSEVAULT_TEST_FAST_KDF"] = "1"

    run_samples = [run_once(binary, args, environment) for _ in range(args.runs)]
    stages = list(run_samples[0]["elapsed_seconds"])
    first = run_samples[0]
    for sample in run_samples[1:]:
        for field in ("vault_bytes", "vault_bytes_after_delete", "deleted_files", "remaining_files", "verified_files"):
            if sample[field] != first[field]:
                raise RuntimeError(f"workload changed between runs: {field}")

    print(
        json.dumps(
            {
                "files": args.files,
                "input_bytes": args.files * args.bytes_per_file,
                "vault_bytes": first["vault_bytes"],
                "vault_bytes_after_delete": first["vault_bytes_after_delete"],
                "profile": args.profile,
                "content": args.content,
                "deleted_files": first["deleted_files"],
                "remaining_files": first["remaining_files"],
                "run_count": args.runs,
                "statistic": "median",
                "elapsed_seconds": {
                    stage: median_for_stage(run_samples, "elapsed_seconds", stage)
                    for stage in stages
                },
                "sampled_peak_rss_bytes": {
                    stage: median_for_stage(
                        run_samples, "sampled_peak_rss_bytes", stage, round_result=False
                    )
                    for stage in stages
                },
                "verified_files": first["verified_files"],
                "run_samples": run_samples,
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
