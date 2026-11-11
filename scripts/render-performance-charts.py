#!/usr/bin/env python3
"""Render the static, GitHub-safe SVG charts from docs/performance/benchmarks.json."""

from __future__ import annotations

import html
import argparse
import json
from pathlib import Path
import sys


ROOT = Path(__file__).resolve().parents[1]
DATA_PATH = ROOT / "docs" / "performance" / "benchmarks.json"
OUTPUT_DIR = ROOT / "docs" / "assets"

BG = "#0b1117"
TEXT = "#edf3f6"
MUTED = "#a6b4bd"
DIM = "#8798a3"
GRID = "#30404b"
CYAN = "#22d3d5"
BLUE = "#77aef7"
GRAY = "#a7b3bc"
FONT = "Segoe UI, Arial, sans-serif"


def esc(value: object) -> str:
    return html.escape(str(value), quote=True)


def text(x: float, y: float, value: object, *, size: int = 16, fill: str = TEXT,
         weight: int = 400, anchor: str = "start", family: str = FONT) -> str:
    return (
        f'<text x="{x:.1f}" y="{y:.1f}" fill="{fill}" '
        f'font-family="{family}" font-size="{size}" font-weight="{weight}" '
        f'text-anchor="{anchor}">{esc(value)}</text>'
    )


def line(x1: float, y1: float, x2: float, y2: float, *, color: str = GRID,
         width: float = 1, dash: str = "") -> str:
    dash_attr = f' stroke-dasharray="{dash}"' if dash else ""
    return (
        f'<line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" '
        f'stroke="{color}" stroke-width="{width}"{dash_attr}/>'
    )


def rect(x: float, y: float, width: float, height: float, fill: str,
         radius: int = 3) -> str:
    return (
        f'<rect x="{x:.1f}" y="{y:.1f}" width="{width:.1f}" height="{height:.1f}" '
        f'rx="{radius}" fill="{fill}"/>'
    )


def document(title: str, description: str, width: int, height: int, body: str) -> str:
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}"
     role="img" aria-labelledby="title desc" aria-label="{esc(title)}">
  <title id="title">{esc(title)}</title>
  <desc id="desc">{esc(description)}</desc>
  <rect width="100%" height="100%" fill="{BG}"/>
  {body}
</svg>
'''


def render_cli(data: dict) -> str:
    width, height = 1440, 750
    stages = data["cli"]["stages"]
    workloads = data["cli"]["workloads"]
    parts = [
        text(48, 51, "CLI operation duration", size=30, weight=700),
        text(48, 82, "Measured with the real CLI · restored paths and SHA-256 digests verified",
             size=16, fill=MUTED),
    ]
    panels = [
        (48, "repeated", "1,000 × 4 KiB repeated", "6-second scale", 6, 1),
        (748, "random", "8 × 10 MiB random", "0.5-second scale", 0.5, 0.1),
    ]
    for panel_x, key, label, scale_label, maximum, step in panels:
        workload = workloads[key]
        parts.append(text(panel_x, 137, label, size=21, weight=650))
        parts.append(text(panel_x, 164,
                          f'{workload["files"]:,} files · {workload["inputBytes"] / 1e6:.3f} MB input · {workload["vaultBytes"] / 1e6:.3f} MB vault',
                          size=14, fill=MUTED))
        parts.append(text(panel_x + 624, 137, scale_label, size=13, fill=DIM, anchor="end"))

        label_width = 146
        bar_x = panel_x + label_width
        bar_width = 407
        value_x = bar_x + bar_width + 17
        y_start = 218
        row_gap = 60
        for index, stage in enumerate(stages):
            y = y_start + index * row_gap
            duration = stage["seconds"][key]
            bar_end = bar_x + (duration / maximum) * bar_width
            parts.append(text(panel_x + label_width - 14, y + 6, stage["name"],
                              size=15, fill=TEXT, anchor="end"))
            parts.append(line(bar_x, y - 9, bar_x, y + 18, color=GRID, width=1))
            parts.append(rect(bar_x, y - 3, max(2, bar_end - bar_x), 17, CYAN))
            parts.append(text(value_x, y + 10, f'{duration:.3f} s', size=15,
                              fill=TEXT, weight=650))

        axis_y = y_start + (len(stages) - 1) * row_gap + 38
        divisions = int(round(maximum / step))
        for tick_index in range(divisions + 1):
            tick = tick_index * step
            x = bar_x + (tick / maximum) * bar_width
            parts.append(line(x, 195, x, axis_y - 15, color=GRID, width=1))
            label_text = f"{tick:.1f}" if step < 1 else f"{tick:.0f}"
            parts.append(text(x, axis_y, label_text, size=12, fill=DIM, anchor="middle"))
        parts.append(text(bar_x + bar_width / 2, axis_y + 28, "Duration (seconds)",
                          size=13, fill=MUTED, anchor="middle"))

    parts.extend([
        line(720, 116, 720, 611, color=GRID, width=1),
        line(48, 692, 1392, 692, color=GRID),
        text(48, 727, "Axes are scaled separately; compare the labels and workload details, not bar lengths across panels.",
             size=14, fill=MUTED),
        text(1392, 727, f'Measured {data["measuredOn"]} · Standard profile · synthetic workloads',
             size=13, fill=DIM, anchor="end"),
    ])
    return document(
        "CLI operation duration by workload",
        "Two verified CLI workloads shown on separate, clearly labeled scales. The repeated workload contains 1,000 4 KiB files. The random workload contains eight 10 MiB files. Each bar is an operation time in seconds.",
        width, height, "\n  ".join(parts),
    )


def render_rss(data: dict) -> str:
    width, height = 1440, 560
    stages = data["cli"]["stages"]
    plot_left, plot_right, plot_top, plot_bottom = 120, 1365, 142, 406
    y_min, y_max = 40, 80
    xs = [plot_left + i * (plot_right - plot_left) / (len(stages) - 1)
          for i in range(len(stages))]

    def y_for(value: float) -> float:
        return plot_bottom - (value - y_min) / (y_max - y_min) * (plot_bottom - plot_top)

    parts = [
        text(48, 51, "Peak resident memory by CLI operation", size=30, weight=700),
        text(48, 82, "Sampled RSS · decimal MB · sampled every 20 ms · synthetic CLI workloads",
             size=16, fill=MUTED),
    ]
    for tick in (40, 50, 60, 70, 80):
        yy = y_for(tick)
        parts.append(line(plot_left, yy, plot_right, yy, color=GRID))
        parts.append(text(plot_left - 15, yy + 5, str(tick), size=13, fill=DIM, anchor="end"))
    offsets: dict[int, dict[str, int]] = {}
    for index, stage in enumerate(stages):
        if stage["rssMB"]["repeated"] >= stage["rssMB"]["random"]:
            offsets[index] = {"repeated": -11, "random": 22}
        else:
            offsets[index] = {"random": -11, "repeated": 22}
    for key, color in (("repeated", CYAN), ("random", BLUE)):
        coords = [(xs[i], y_for(stage["rssMB"][key])) for i, stage in enumerate(stages)]
        point_string = " ".join(f"{x:.1f},{yy:.1f}" for x, yy in coords)
        parts.append(f'<polyline points="{point_string}" fill="none" stroke="{color}" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>')
        for index, ((x, yy), stage) in enumerate(zip(coords, stages)):
            value = stage["rssMB"][key]
            parts.append(f'<circle cx="{x:.1f}" cy="{yy:.1f}" r="6" fill="{color}" stroke="{BG}" stroke-width="2"/>')
            parts.append(text(x, yy + offsets[index][key], f"{value:.1f}", size=14, fill=TEXT,
                              weight=650, anchor="middle"))

    for x, stage in zip(xs, stages):
        parts.append(line(x, plot_bottom, x, plot_bottom + 7, color=GRID))
        parts.append(text(x, plot_bottom + 31, stage["name"], size=14, fill=MUTED,
                          anchor="middle"))

    parts.extend([
        rect(483, 487, 18, 12, CYAN, 2),
        text(510, 499, "1,000 × 4 KiB repeated", size=14, fill=TEXT),
        rect(735, 487, 18, 12, BLUE, 2),
        text(762, 499, "8 × 10 MiB random", size=14, fill=TEXT),
        line(48, 528, 1392, 528, color=GRID),
        text(48, 550, "RSS is sampled every 20 ms. Short memory peaks can fall between samples; use these as approximate stage peaks.",
             size=13, fill=MUTED),
    ])
    return document(
        "Sampled peak RSS by CLI operation",
        "Line chart of approximate peak resident set size in decimal megabytes at each of seven CLI stages, including batch delete and a post-delete verification. Cyan is the repeated-data workload; blue is random data. Memory is sampled every 20 milliseconds, so short peaks can be missed.",
        width, height, "\n  ".join(parts),
    )


def render_search(data: dict) -> str:
    width, height = 1440, 820
    groups = data["search"]
    methods = [
        ("legacy", "Legacy filter", GRAY),
        ("indexedRepeat", "Indexed repeat", CYAN),
        ("indexBuild", "One-time index build", BLUE),
    ]
    max_ms = 30
    plot_top, plot_bottom = 170, 442
    plot_height = plot_bottom - plot_top
    centers = [455, 1010]
    group_width = 385
    bar_width = 78
    gap = 27
    parts = [
        text(48, 51, "Search time and index construction", size=30, weight=700),
        text(48, 82, "Case-insensitive path filtering · elapsed milliseconds · lower is faster",
             size=16, fill=MUTED),
    ]

    for tick in (0, 5, 10, 15, 20, 25, 30):
        yy = plot_bottom - tick / max_ms * plot_height
        parts.append(line(157, yy, 1348, yy, color=GRID))
        parts.append(text(139, yy + 5, str(tick), size=13, fill=DIM, anchor="end"))
    parts.append(text(48, 285, "Time (ms)", size=13, fill=MUTED))

    for group_index, group in enumerate(groups):
        center = centers[group_index]
        start_x = center - group_width / 2
        for method_index, (key, _, color) in enumerate(methods):
            x = start_x + method_index * (bar_width + gap) + (group_width - (3 * bar_width + 2 * gap)) / 2
            ms = group[key]["ns"] / 1e6
            bar_height = ms / max_ms * plot_height
            y = plot_bottom - bar_height
            parts.append(rect(x, y, bar_width, bar_height, color, 3))
            parts.append(text(x + bar_width / 2, max(plot_top - 13, y - 9), f"{ms:.2f} ms",
                              size=13, fill=TEXT, weight=650, anchor="middle"))
        parts.append(text(center, plot_bottom + 32, f'{group["paths"] // 1000}k paths',
                          size=17, fill=TEXT, weight=650, anchor="middle"))

    legend_x = [347, 550, 805]
    for (key, label, color), x in zip(methods, legend_x):
        parts.append(rect(x, 497, 17, 13, color, 2))
        parts.append(text(x + 27, 510, label, size=14, fill=TEXT))

    table_top = 555
    parts.extend([
        line(48, 535, 1392, 535, color=GRID),
        text(48, table_top, "Allocation cost per operation", size=18, weight=650),
    ])
    columns = [48, 245, 553, 854, 1132]
    headers = ["Path count", "Method", "Time", "Bytes/op", "Allocs/op"]
    for x, header in zip(columns, headers):
        parts.append(text(x, table_top + 36, header, size=13, fill=MUTED, weight=650))
    row_y = table_top + 69
    for group in groups:
        for key, label, _ in methods:
            measurement = group[key]
            parts.append(line(48, row_y - 17, 1392, row_y - 17, color=GRID))
            parts.append(text(columns[0], row_y, f'{group["paths"]:,}', size=14))
            parts.append(text(columns[1], row_y, label, size=14))
            parts.append(text(columns[2], row_y, f'{measurement["ns"] / 1e6:.2f} ms', size=14))
            parts.append(text(columns[3], row_y, f'{measurement["bytes"]:,}', size=14))
            parts.append(text(columns[4], row_y, f'{measurement["allocations"]:,}', size=14))
            row_y += 29
    parts.append(line(48, row_y - 17, 1392, row_y - 17, color=GRID))
    return document(
        "Path search time and allocation costs",
        "Grouped bars compare legacy case-insensitive filtering, repeat queries reusing the case-folded index, and the one-time index build at 10,000 and 100,000 paths. The table reports elapsed milliseconds, allocated bytes, and allocations per operation. The chart covers filtering code only, not Fyne rendering.",
        width, height, "\n  ".join(parts),
    )


def render_conflict(data: dict) -> str:
    width, height = 1200, 490
    conflict = data["pathConflict"]
    baseline = conflict["nestedScanNs"] / 1e6
    indexed = conflict["foldedIndexNs"] / 1e6
    maximum = max(60, ((int(baseline) + 59) // 60) * 60)
    tick_step = maximum // 6
    x0, plot_width = 320, 710
    rows = [
        ("Nested scan", baseline, GRAY),
        ("Unicode-folded index", indexed, CYAN),
    ]
    parts = [
        text(46, 52, "Bulk-add path conflict check", size=29, weight=700),
        text(46, 82, f'{conflict["existingPaths"]:,} existing paths · {conflict["incomingPaths"]:,} incoming paths · lower is faster',
             size=16, fill=MUTED),
    ]
    for tick in range(0, maximum + 1, tick_step):
        x = x0 + tick / maximum * plot_width
        parts.append(line(x, 145, x, 333, color=GRID))
        parts.append(text(x, 358, str(tick), size=13, fill=DIM, anchor="middle"))
    for i, (label, value, color) in enumerate(rows):
        y = 199 + i * 92
        parts.append(text(x0 - 20, y + 6, label, size=16, anchor="end"))
        parts.append(rect(x0, y - 12, value / maximum * plot_width, 24, color))
        parts.append(text(x0 + value / maximum * plot_width + 14, y + 6,
                          f"{value:.2f} ms", size=16, weight=650))
    parts.extend([
        text(x0 + plot_width / 2, 393, "Validation time (milliseconds)", size=14,
             fill=MUTED, anchor="middle"),
        line(46, 419, 1154, 419, color=GRID),
        text(46, 457, f'About {baseline / indexed:.1f}× faster', size=23, fill=CYAN, weight=700),
        text(410, 457,
             f'{conflict["foldedIndexBytes"] / 1e6:.2f} MB · {conflict["foldedIndexAllocations"]:,} allocations · released after add',
             size=15, fill=MUTED),
    ])
    return document(
        "Bulk-add path conflict validation time",
        f'At {conflict["existingPaths"]:,} existing and {conflict["incomingPaths"]:,} incoming paths, the former nested comparison took {baseline:.2f} milliseconds and a Unicode-aware folded-path index took {indexed:.2f} milliseconds. The index used {conflict["foldedIndexBytes"] / 1e6:.2f} decimal MB, made {conflict["foldedIndexAllocations"]:,} temporary allocations, and is released after add.',
        width, height, "\n  ".join(parts),
    )


def render_archive_lookup(data: dict) -> str:
    width, height = 1440, 690
    comparison = data["bulkArchiveLookup"]
    workload_key = comparison["workloadKey"]
    workload = data["cli"]["workloads"][workload_key]
    current = {stage["name"]: stage for stage in data["cli"]["stages"]}
    max_seconds = 6
    x0, plot_width = 350, 760
    rows = []
    for entry in comparison["stages"]:
        now = current[entry["name"]]
        before = entry["beforeSeconds"]
        after = now["seconds"][workload_key]
        reduction = (1 - after / before) * 100
        rows.append((entry["name"], before, after, reduction))

    parts = [
        text(48, 52, "Bulk verification and restore: before and after", size=30, weight=700),
        text(48, 84,
             f'{workload["label"]} · standard password profile · median of {comparison["runs"]} complete CLI runs',
             size=16, fill=MUTED),
        text(48, 119, "Operation time · common 0–6 second scale", size=14, fill=MUTED),
    ]
    for index, tick in enumerate(range(0, max_seconds + 1)):
        x = x0 + tick / max_seconds * plot_width
        parts.append(line(x, 143, x, 476, color=GRID))
        parts.append(text(x, 134, str(tick), size=12, fill=DIM, anchor="middle"))

    for index, (name, before, after, reduction) in enumerate(rows):
        group_y = 173 + index * 104
        parts.append(text(48, group_y + 41, name, size=16, weight=650))
        for offset, label, value, color in [
            (20, "Before shared index", before, GRAY),
            (52, "After shared index", after, CYAN),
        ]:
            y = group_y + offset
            parts.append(text(x0 - 17, y + 5, label, size=13, fill=MUTED, anchor="end"))
            bar_width = value / max_seconds * plot_width
            parts.append(rect(x0, y - 9, bar_width, 18, color, 3))
            parts.append(text(x0 + bar_width + 10, y + 5, f"{value:.3f} s", size=14, weight=650))
        parts.append(text(1260, group_y + 42, f"{reduction:.1f}% less", size=17,
                          fill=CYAN, weight=700))

    before_rss = comparison["extractRssMBBefore"]
    after_rss = current["Extract all"]["rssMB"][workload_key]
    rss_max = 80
    rss_x0, rss_width = 350, 760
    parts.extend([
        line(48, 500, 1392, 500, color=GRID),
        text(48, 540, "Sampled peak RSS during extract-all", size=18, weight=650),
        text(48, 566, "Separate 0–80 decimal MB scale", size=13, fill=MUTED),
    ])
    for tick in range(0, rss_max + 1, 20):
        x = rss_x0 + tick / rss_max * rss_width
        parts.append(line(x, 552, x, 648, color=GRID))
        parts.append(text(x, 545, str(tick), size=11, fill=DIM, anchor="middle"))
    for y, label, value, color in [
        (584, "Before shared index", before_rss, GRAY),
        (618, "After shared index", after_rss, BLUE),
    ]:
        parts.append(text(rss_x0 - 17, y + 5, label, size=13, fill=MUTED, anchor="end"))
        bar_width = value / rss_max * rss_width
        parts.append(rect(rss_x0, y - 9, bar_width, 18, color, 3))
        parts.append(text(rss_x0 + bar_width + 10, y + 5, f"{value:.1f} MB", size=14, weight=650))
    parts.append(text(1392, 668, "RSS sampled every 20 ms; short-lived peaks may be missed.",
                      size=12, fill=DIM, anchor="end"))

    description = "; ".join(
        f"{name}: {before:.3f} seconds before and {after:.3f} seconds after ({reduction:.1f}% less time)"
        for name, before, after, reduction in rows
    )
    return document(
        "Bulk archive lookup performance before and after indexing",
        f"{workload['label']} with the standard password profile. Medians from {comparison['runs']} complete CLI runs. {description}. Extract-all sampled peak RSS was {before_rss:.1f} decimal MB before and {after_rss:.1f} decimal MB after; RSS was sampled every 20 milliseconds.",
        width, height, "\n  ".join(parts),
    )


def render_zip_validation(data: dict) -> str:
    width, height = 1440, 530
    benchmark = data["zipValidation"]
    maximum_us = 900
    x0, plot_width = 345, 620
    parts = [
        text(48, 52, "ZIP entry validation: time and allocation cost", size=30, weight=700),
        text(48, 84,
             f"Go microbenchmark · {benchmark['runs']} × {benchmark['benchmarkDurationMs']} ms runs · median per operation",
             size=16, fill=MUTED),
        text(x0, 122, "Time per validation · common 0–900 µs scale", size=14, fill=MUTED),
        text(1010, 122, "Allocation change", size=14, fill=MUTED),
    ]
    for tick in range(0, maximum_us + 1, 150):
        x = x0 + tick / maximum_us * plot_width
        parts.append(line(x, 143, x, 420, color=GRID))
        parts.append(text(x, 137, str(tick), size=12, fill=DIM, anchor="middle"))

    for index, result in enumerate(benchmark["results"]):
        group_y = 175 + index * 139
        before = result["before"]
        after = result["after"]
        before_us = before["nsPerOp"] / 1_000
        after_us = after["nsPerOp"] / 1_000
        speedup = before_us / after_us
        allocation_reduction = (1 - after["allocationsPerOp"] / before["allocationsPerOp"]) * 100
        parts.append(text(48, group_y + 30, f'{result["entries"]:,} entries', size=18, weight=650))
        for offset, label, value, color in [
            (17, "Before", before_us, GRAY),
            (55, "After", after_us, CYAN),
        ]:
            y = group_y + offset
            parts.append(text(x0 - 16, y + 5, label, size=14, fill=MUTED, anchor="end"))
            bar_width = value / maximum_us * plot_width
            parts.append(rect(x0, y - 9, bar_width, 17, color, 3))
            parts.append(text(x0 + bar_width + 9, y + 5, f"{value:.1f} µs", size=14, weight=650))
        parts.append(text(1010, group_y + 25, f"{speedup:.2f}× faster", size=18, fill=CYAN, weight=700))
        parts.append(text(1010, group_y + 52,
                         f'{before["allocationsPerOp"]:,} → {after["allocationsPerOp"]:,} allocations/op',
                         size=14, fill=MUTED))
        parts.append(text(1010, group_y + 75,
                         f'{before["bytesPerOp"]:,} → {after["bytesPerOp"]:,} bytes/op',
                         size=13, fill=DIM))

    parts.extend([
        line(48, 450, 1392, 450, color=GRID),
        text(48, 485,
             "Validation only; excludes ZIP opening and disk I/O. Path checks now scan segments without allocating a split slice per entry.",
             size=14, fill=MUTED),
    ])
    return document(
        "ZIP entry validation time and allocations before and after path-scan optimization",
        f"Direct validateZip Go microbenchmarks over synthetic stored ZIP entries, excluding archive opening and disk I/O. Medians from {benchmark['runs']} {benchmark['benchmarkDurationMs']} millisecond runs. At 1,000 entries, median time changed from {benchmark['results'][0]['before']['nsPerOp'] / 1e3:.1f} to {benchmark['results'][0]['after']['nsPerOp'] / 1e3:.1f} microseconds; allocations per operation changed from {benchmark['results'][0]['before']['allocationsPerOp']} to {benchmark['results'][0]['after']['allocationsPerOp']}. At 10,000 entries, median time changed from {benchmark['results'][1]['before']['nsPerOp'] / 1e3:.1f} to {benchmark['results'][1]['after']['nsPerOp'] / 1e3:.1f} microseconds; allocations per operation changed from {benchmark['results'][1]['before']['allocationsPerOp']} to {benchmark['results'][1]['after']['allocationsPerOp']}.",
        width, height, "\n  ".join(parts),
    )


def render_crypto(data: dict) -> str:
    width, height = 1440, 570
    crypto = data["crypto"]
    samples = crypto["v6Stream"]
    repetitions = crypto["repetitions"]
    duration = crypto["benchmarkDurationMs"]
    panels = [
        (48, "Encryption time", 12, 2, "Time per operation (milliseconds)",
         lambda sample: sample["medianNs"] / 1e6, "{:.2f} ms", CYAN),
        (744, "Allocated memory", 6, 1, "Go allocations per operation (decimal MB)",
         lambda sample: sample["bytesPerOp"] / 1e6, "{:.2f} MB", BLUE),
    ]
    parts = [
        text(48, 50, "V6 stream encryption: time and allocations", size=29, weight=700),
        text(48, 81, f"Encrypt-only microbenchmark · {repetitions} × {duration} ms runs · median per operation",
             size=15, fill=MUTED),
    ]
    for panel_x, title, maximum, step, unit, value_for, value_format, color in panels:
        label_width = 166
        bar_x = panel_x + label_width
        bar_width = 405
        parts.append(text(panel_x, 132, title, size=20, weight=650))
        parts.append(text(panel_x + 642, 132, "lower is faster / smaller",
                          size=13, fill=DIM, anchor="end"))
        for index, sample in enumerate(samples):
            y = 205 + index * 74
            value = value_for(sample)
            end = bar_x + value / maximum * bar_width
            parts.append(text(bar_x - 13, y + 6, sample["label"], size=14,
                              fill=TEXT, anchor="end"))
            parts.append(rect(bar_x, y - 3, max(2, end - bar_x), 17, color))
            parts.append(text(bar_x + bar_width + 12, y + 9,
                              value_format.format(value), size=14, weight=650))
        axis_y = 432
        for tick in range(0, maximum + 1, step):
            x = bar_x + tick / maximum * bar_width
            parts.append(line(x, 165, x, axis_y - 13, color=GRID))
            parts.append(text(x, axis_y + 9, str(tick), size=12, fill=DIM, anchor="middle"))
        parts.append(text(bar_x + bar_width / 2, axis_y + 35, unit,
                          size=12, fill=MUTED, anchor="middle"))
    parts.extend([
        line(720, 113, 720, 486, color=GRID),
        line(48, 510, 1392, 510, color=GRID),
        text(48, 541,
             "Synthetic repeated and random inputs. Bytes/op are Go heap allocations, not peak process RSS.",
             size=14, fill=MUTED),
        text(1392, 541,
             "Compression enabled when beneficial · 1 MiB stream chunks",
             size=13, fill=DIM, anchor="end"),
    ])
    return document(
        "V6 stream encryption time and allocated memory",
        f"Two panels compare median encryption latency in milliseconds and Go allocated bytes per operation for three synthetic workloads: a 4.14 KiB repeated payload, an approximately 1 MiB repeated payload, and a 10 MiB random payload. Each median is from {repetitions} {duration} millisecond benchmark runs. Allocation bytes are not peak process RSS.",
        width, height, "\n  ".join(parts),
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true",
                        help="fail when generated SVGs do not match the benchmark data")
    args = parser.parse_args()
    data = json.loads(DATA_PATH.read_text(encoding="utf-8"))
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    charts = {
        "performance-cli-time.svg": render_cli(data),
        "performance-cli-memory.svg": render_rss(data),
        "performance-search.svg": render_search(data),
        "performance-path-index.svg": render_conflict(data),
        "performance-bulk-archive.svg": render_archive_lookup(data),
        "performance-zip-validation.svg": render_zip_validation(data),
        "performance-stream.svg": render_crypto(data),
    }
    stale = []
    for filename, content in charts.items():
        path = OUTPUT_DIR / filename
        if args.check:
            if not path.exists() or path.read_text(encoding="utf-8") != content:
                stale.append(filename)
        else:
            path.write_text(content, encoding="utf-8", newline="\n")
    if stale:
        print("Generated benchmark charts are stale: " + ", ".join(stale), file=sys.stderr)
        print("Run: python scripts/render-performance-charts.py", file=sys.stderr)
        return 1
    mode = "Checked" if args.check else "Rendered"
    print(f"{mode} {len(charts)} benchmark charts from {DATA_PATH.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
