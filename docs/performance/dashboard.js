const workloadInputs = [...document.querySelectorAll('input[name="workload"]')];
const stageSlider = document.querySelector('#stage-slider');
const stageOutput = document.querySelector('#stage-output');

let benchmarkData;
let selectedWorkload = 'repeated';
let selectedStage = Number(stageSlider.value);

const number = (value, digits = 2) => Number(value).toLocaleString('en-US', {
  minimumFractionDigits: digits,
  maximumFractionDigits: digits,
});

const tickMarkup = (ticks, unit = '') => ticks.map((tick) =>
  `<span>${number(tick, tick % 1 === 0 ? 0 : 1)}${unit}</span>`
).join('');

function renderOperationChart() {
  const { stages, workloads } = benchmarkData.cli;
  stageSlider.max = String(stages.length - 1);
  const current = workloads[selectedWorkload];
  const otherKey = selectedWorkload === 'repeated' ? 'random' : 'repeated';
  const other = workloads[otherKey];
  const max = selectedWorkload === 'repeated' ? 6 : 0.5;
  const ticks = selectedWorkload === 'repeated' ? [0, 1, 2, 3, 4, 5, 6] : [0, 0.1, 0.2, 0.3, 0.4, 0.5];
  const tickStep = `${100 / (ticks.length - 1)}%`;
  const rows = stages.map((stage, index) => {
    const seconds = stage.seconds[selectedWorkload];
    const selected = index === selectedStage;
    return `<button type="button" class="bar-row" data-stage="${index}" aria-pressed="${selected}" aria-label="Select ${stage.name}; ${number(seconds, 3)} seconds for ${current.label}">
      <span class="bar-name">${stage.name}</span>
      <span class="bar-track" style="--tick-step:${tickStep}"><span class="bar-fill" style="width:${(seconds / max * 100).toFixed(2)}%"></span></span>
      <span class="bar-value">${number(seconds, 3)} s</span>
    </button>`;
  }).join('');
  const axis = `<div class="axis-row" aria-hidden="true"><span></span><div class="axis-ticks">${tickMarkup(ticks)}</div><span></span><span class="axis-unit">Duration (seconds)</span></div>`;

  document.querySelector('#operation-chart').innerHTML = rows + axis;
  document.querySelectorAll('#operation-chart .bar-row').forEach((button) => {
    button.addEventListener('click', () => setStage(Number(button.dataset.stage)));
  });

  const selected = stages[selectedStage];
  const selectedTime = selected.seconds[selectedWorkload];
  document.querySelector('#workload-summary').textContent = `${current.label} · ${number(current.inputBytes / 1e6, 3)} MB input · each workload uses its own axis scale.`;
  document.querySelector('#operation-caption').textContent = `Vault: ${number(current.vaultBytes / 1e6, 3)} MB before delete → ${number(current.vaultBytesAfterDelete / 1e6, 3)} MB after. ${current.files.toLocaleString('en-US')} restored files passed SHA-256 checks; ${current.deletedFiles.toLocaleString('en-US')} entries were removed in one rewrite, then ${current.files - current.deletedFiles} remaining entries were verified.`;
  document.querySelector('#selected-stage-name').textContent = selected.name;
  document.querySelector('#selected-workload-name').textContent = `Time · ${current.label}`;
  document.querySelector('#selected-time').textContent = `${number(selectedTime, 3)} s`;
  document.querySelector('#other-workload-name').textContent = other.label;
  document.querySelector('#other-workload-time').textContent = `${number(selected.seconds[otherKey], 3)} s`;
  document.querySelector('#stage-output').textContent = selected.name;
  stageSlider.setAttribute('aria-valuetext', selected.name);
  document.querySelectorAll('#stage-labels button').forEach((button, index) => {
    button.setAttribute('aria-pressed', String(index === selectedStage));
  });
}

function setStage(index) {
  selectedStage = Math.max(0, Math.min(benchmarkData.cli.stages.length - 1, index));
  stageSlider.value = selectedStage;
  renderOperationChart();
}

function renderStageControls() {
  const labels = benchmarkData.cli.stages.map((stage, index) =>
    `<button type="button" data-stage="${index}" aria-label="Select ${stage.name}" aria-pressed="${index === selectedStage}">${stage.shortName ?? stage.name}</button>`
  ).join('');
  document.querySelector('#stage-labels').innerHTML = labels;
  document.querySelectorAll('#stage-labels button').forEach((button) => {
    button.addEventListener('click', () => setStage(Number(button.dataset.stage)));
  });
}

function renderRssChart() {
  const { stages } = benchmarkData.cli;
  const x = stages.map((_, index) => stages.length === 1 ? 276 : 18 + index * (516 / (stages.length - 1)));
  const y = (value) => 16 + (80 - value) / 40 * 174;
  const pointsFor = (key) => stages.map((stage, index) => `${x[index]},${y(stage.rssMB[key])}`).join(' ');
  const grid = [40, 50, 60, 70, 80].map((value) => {
    const gy = y(value);
    return `<line class="rss-gridline" x1="0" y1="${gy}" x2="552" y2="${gy}" />`;
  }).join('');
  const marksFor = (key, className) => stages.map((stage, index) =>
    `<circle class="rss-point ${className}" cx="${x[index]}" cy="${y(stage.rssMB[key])}" r="5" />`
  ).join('');
  const rangeFor = (key) => {
    const values = stages.map((stage) => stage.rssMB[key]);
    return `${number(Math.min(...values), 1)} to ${number(Math.max(...values), 1)}`;
  };

  const stageLabels = stages.map((stage) => `<span>${stage.shortName ?? stage.name}</span>`).join('');
  document.querySelector('#rss-chart').innerHTML = `
    <div class="rss-y-axis" aria-hidden="true"><span>80</span><span>70</span><span>60</span><span>50</span><span>40</span></div>
    <div class="rss-plot">
      <svg viewBox="0 0 552 206" role="img" aria-labelledby="rss-svg-title rss-svg-desc" preserveAspectRatio="none">
        <title id="rss-svg-title">Peak resident memory at each CLI stage</title>
        <desc id="rss-svg-desc">The repeated workload ranges from ${rangeFor('repeated')} decimal megabytes. The random workload ranges from ${rangeFor('random')} megabytes. Measurements are sampled every 20 milliseconds.</desc>
        ${grid}
        <polyline class="rss-line rss-line-repeated" points="${pointsFor('repeated')}" />
        <polyline class="rss-line rss-line-random" points="${pointsFor('random')}" />
        ${marksFor('repeated', 'rss-point-repeated')}
        ${marksFor('random', 'rss-point-random')}
      </svg>
    </div>
    <div class="rss-stage-labels" aria-hidden="true">${stageLabels}</div>
    <div class="legend"><span><i class="swatch swatch-repeated"></i>1,000 × 4 KiB repeated</span><span><i class="swatch swatch-random"></i>8 × 10 MiB random</span></div>`;

  document.querySelector('#rss-table').innerHTML = `
    <thead><tr><th scope="col">Stage</th><th scope="col">1,000 × 4 KiB<br>repeated</th><th scope="col">8 × 10 MiB<br>random</th></tr></thead>
    <tbody>${stages.map((stage) => `<tr><th scope="row">${stage.name}</th><td>${number(stage.rssMB.repeated, 1)} MB</td><td>${number(stage.rssMB.random, 1)} MB</td></tr>`).join('')}</tbody>`;
}

function renderSearch() {
  const groups = benchmarkData.search;
  const methods = [
    { key: 'legacy', label: 'Legacy filter', color: 'legacy' },
    { key: 'indexedRepeat', label: 'Indexed repeat', color: 'indexed' },
    { key: 'indexBuild', label: 'Index build', color: 'build' },
  ];
  const max = 30;
  const ticks = [0, 5, 10, 15, 20, 25, 30];
  const tickStep = `${100 / (ticks.length - 1)}%`;
  const charts = groups.map((group) => `<section class="search-group" aria-label="${group.paths.toLocaleString('en-US')} path names">
    <h4>${group.paths.toLocaleString('en-US')} paths</h4>
    ${methods.map((method) => {
      const measurement = group[method.key];
      const milliseconds = measurement.ns / 1e6;
      return `<div class="bar-row">
        <span class="bar-name">${method.label}</span>
        <span class="bar-track" style="--tick-step:${tickStep}" aria-hidden="true"><span class="bar-fill ${method.color}" style="width:${Math.max(0.8, milliseconds / max * 100).toFixed(2)}%"></span></span>
        <span class="bar-value">${number(milliseconds, 2)} ms</span>
      </div>`;
    }).join('')}
  </section>`).join('');
  const axis = `<div class="search-axis" aria-hidden="true"><span></span><div class="axis-ticks">${tickMarkup(ticks)}</div><span></span><span class="axis-unit">Elapsed time (milliseconds)</span></div>`;
  document.querySelector('#search-chart').innerHTML = charts + axis;
  const large = groups.reduce((largest, group) => group.paths > largest.paths ? group : largest, groups[0]);
  const breakEven = Math.floor(large.indexBuild.ns / (large.legacy.ns - large.indexedRepeat.ns)) + 1;
  document.querySelector('#search-summary').textContent = `At ${large.paths.toLocaleString('en-US')} paths, legacy filtering took ${number(large.legacy.ns / 1e6, 2)} ms; a repeated indexed query took ${number(large.indexedRepeat.ns / 1e6, 2)} ms after a ${number(large.indexBuild.ns / 1e6, 2)} ms index build. The build cost is offset after about ${breakEven} searches.`;

  document.querySelector('#search-table').innerHTML = `
    <thead><tr><th scope="col">Paths</th><th scope="col">Method</th><th scope="col">Time</th><th scope="col">Bytes/op</th><th scope="col">Allocs/op</th></tr></thead>
    <tbody>${groups.map((group) => methods.map((method) => {
      const measurement = group[method.key];
      return `<tr><th scope="row">${group.paths.toLocaleString('en-US')}</th><td>${method.label}</td><td>${number(measurement.ns / 1e6, 2)} ms</td><td>${measurement.bytes.toLocaleString('en-US')}</td><td>${measurement.allocations.toLocaleString('en-US')}</td></tr>`;
    }).join('')).join('')}</tbody>`;

  const row = benchmarkData.guiRowUpdate;
  document.querySelector('#gui-row-update-time').textContent = `${number(row.medianNsPerOp / 1000, 3)} µs/op`;
  document.querySelector('#gui-row-update-cost').textContent = `${row.bytesPerOp.toLocaleString('en-US')} B/op · ${row.allocationsPerOp} allocations/op`;
  document.querySelector('#gui-row-update-scope').textContent = `Median of ${row.runs} one-second runs. ${row.scope}.`;
}

function renderCrypto() {
  const samples = benchmarkData.crypto.v6Stream;
  const timeValues = samples.map((sample) => sample.medianNs / 1e6);
  const allocationValues = samples.map((sample) => sample.bytesPerOp / 1e6);
  const scaleFor = (values) => {
    const raw = Math.max(...values) / 5;
    const magnitude = 10 ** Math.floor(Math.log10(raw));
    const normalized = raw / magnitude;
    const factor = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
    const step = factor * magnitude;
    return { step, max: Math.ceil(Math.max(...values) / step) * step };
  };
  const timeScale = scaleFor(timeValues);
  const allocationScale = scaleFor(allocationValues);
  const metrics = [
    { title: 'Median encryption time', values: timeValues, scale: timeScale, unit: 'milliseconds', suffix: ' ms', className: 'time', digits: 2 },
    { title: 'Go allocations per operation', values: allocationValues, scale: allocationScale, unit: 'decimal megabytes', suffix: ' MB', className: 'allocation', digits: 2 },
  ];
  document.querySelector('#crypto-chart').innerHTML = metrics.map((metric) => {
    const ticks = Array.from({length: 6}, (_, index) => index * metric.scale.max / 5);
    const tickStep = `${100 / (ticks.length - 1)}%`;
    const rows = samples.map((sample, index) => {
      const value = metric.values[index];
      const width = Math.max(0.8, value / metric.scale.max * 100);
      return `<div class="bar-row" aria-label="${sample.label}: ${number(value, metric.digits)}${metric.suffix}">
        <span class="bar-name">${sample.label}</span>
        <span class="bar-track" style="--tick-step:${tickStep}" aria-hidden="true"><span class="bar-fill ${metric.className}" style="width:${width.toFixed(2)}%"></span></span>
        <span class="bar-value">${number(value, metric.digits)}${metric.suffix}</span>
      </div>`;
    }).join('');
    const axis = `<div class="crypto-axis" aria-hidden="true"><span></span><div class="axis-ticks">${tickMarkup(ticks)}</div><span></span><span class="axis-unit">${metric.unit}</span></div>`;
    return `<section class="crypto-metric" aria-label="${metric.title}"><h3>${metric.title}</h3>${rows}${axis}</section>`;
  }).join('');

  const small = samples[0];
  const large = samples[2];
  const {benchmarkDurationMs, repetitions} = benchmarkData.crypto;
  document.querySelector('#crypto-summary').textContent = `${repetitions} runs of ${benchmarkDurationMs} ms per case; values are medians. The samples span ${number(small.inputBytes, 0)} bytes to ${number(large.inputBytes / 1e6, 1)} MB of plaintext.`;
}

function renderConflict() {
  const data = benchmarkData.pathConflict;
  const nested = data.nestedScanNs / 1e6;
  const indexed = data.foldedIndexNs / 1e6;
  const max = Math.max(60, Math.ceil(nested / 60) * 60);
  const ticks = Array.from({length: 7}, (_, index) => index * max / 6);
  const tickStep = `${100 / (ticks.length - 1)}%`;
  const rows = [
    { label: 'Nested scan', value: nested, className: 'baseline' },
    { label: 'Unicode-folded index', value: indexed, className: 'optimized' },
  ].map((entry) => `<div class="bar-row">
      <span class="bar-name">${entry.label}</span>
      <span class="bar-track" style="--tick-step:${tickStep}" aria-hidden="true"><span class="bar-fill ${entry.className}" style="width:${(entry.value / max * 100).toFixed(2)}%"></span></span>
      <span class="bar-value">${number(entry.value, 2)} ms</span>
    </div>`).join('');
  const axis = `<div class="axis-row" aria-hidden="true"><span></span><div class="axis-ticks">${tickMarkup(ticks)}</div><span></span><span class="axis-unit">Validation time (milliseconds)</span></div>`;
  document.querySelector('#conflict-input').textContent = `${data.existingPaths.toLocaleString('en-US')} existing names · ${data.incomingPaths.toLocaleString('en-US')} incoming names`;
  document.querySelector('#conflict-chart').innerHTML = rows + axis;
  document.querySelector('#conflict-speedup').textContent = `About ${(nested / indexed).toFixed(1)}× faster`;
  document.querySelector('#conflict-memory').textContent = `${number(data.foldedIndexBytes / 1e6, 2)} MB · ${data.foldedIndexAllocations.toLocaleString('en-US')} temporary allocations · released after add`;
}

function renderArchiveLookup() {
  const comparison = benchmarkData.bulkArchiveLookup;
  const workload = benchmarkData.cli.workloads[comparison.workloadKey];
  const currentByName = new Map(benchmarkData.cli.stages.map((stage) => [stage.name, stage]));
  const rows = comparison.stages.map((entry) => {
    const current = currentByName.get(entry.name);
    const after = current.seconds[comparison.workloadKey];
    const reduction = (1 - after / entry.beforeSeconds) * 100;
    return { name: entry.name, before: entry.beforeSeconds, after, reduction };
  });
  const max = 6;
  const ticks = [0, 1, 2, 3, 4, 5, 6];
  const tickStep = `${100 / (ticks.length - 1)}%`;
  const groups = rows.map((entry) => `<section class="archive-lookup-group" aria-label="${entry.name}: ${number(entry.before, 3)} seconds before shared indexing, ${number(entry.after, 3)} seconds after">
    <h4>${entry.name}<span>${number(entry.reduction, 1)}% less time</span></h4>
    <div class="bar-row">
      <span class="bar-name">Before shared index</span>
      <span class="bar-track" style="--tick-step:${tickStep}" aria-hidden="true"><span class="bar-fill baseline" style="width:${(entry.before / max * 100).toFixed(2)}%"></span></span>
      <span class="bar-value">${number(entry.before, 3)} s</span>
    </div>
    <div class="bar-row">
      <span class="bar-name">After shared index</span>
      <span class="bar-track" style="--tick-step:${tickStep}" aria-hidden="true"><span class="bar-fill indexed" style="width:${(entry.after / max * 100).toFixed(2)}%"></span></span>
      <span class="bar-value">${number(entry.after, 3)} s</span>
    </div>
  </section>`).join('');
  const axis = `<div class="archive-lookup-axis" aria-hidden="true"><span></span><div class="axis-ticks">${tickMarkup(ticks)}</div><span></span><span class="axis-unit">Time per operation (seconds) · common 0–6 s scale</span></div>`;
  document.querySelector('#archive-lookup-chart').innerHTML = groups + axis;
  document.querySelector('#archive-lookup-summary').textContent = `Standard-profile CLI medians from ${comparison.runs} runs on ${workload.label}: verify ${number(rows[0].before / rows[0].after, 2)}× faster; extract-all ${number(rows[1].reduction, 1)}% less time; verify after delete ${number(rows[2].before / rows[2].after, 2)}× faster.`;

  const rssAfter = currentByName.get('Extract all').rssMB[comparison.workloadKey];
  const rssReduction = (1 - rssAfter / comparison.extractRssMBBefore) * 100;
  document.querySelector('#archive-lookup-memory').innerHTML = `Sampled peak RSS during extract-all: ${number(comparison.extractRssMBBefore, 1)} MB before → ${number(rssAfter, 1)} MB after (${number(rssReduction, 1)}% lower).`;
  document.querySelector('#archive-lookup-table').innerHTML = `
    <thead><tr><th scope="col">Operation</th><th scope="col">Before</th><th scope="col">After</th><th scope="col">Less time</th></tr></thead>
    <tbody>${rows.map((entry) => `<tr><th scope="row">${entry.name}</th><td>${number(entry.before, 3)} s</td><td>${number(entry.after, 3)} s</td><td>${number(entry.reduction, 1)}%</td></tr>`).join('')}</tbody>`;
}

function renderZipValidation() {
  const benchmark = benchmarkData.zipValidation;
  const results = benchmark.results;
  const max = 900;
  const ticks = [0, 150, 300, 450, 600, 750, 900];
  const tickStep = `${100 / (ticks.length - 1)}%`;
  const rows = results.map((entry) => {
    const before = entry.before;
    const after = entry.after;
    const speedup = before.nsPerOp / after.nsPerOp;
    const allocationsReduced = (1 - after.allocationsPerOp / before.allocationsPerOp) * 100;
    return { entry, before, after, speedup, allocationsReduced };
  });
  const groups = rows.map(({entry, before, after, speedup, allocationsReduced}) => `<section class="archive-lookup-group" aria-label="${entry.entries.toLocaleString('en-US')} entries: ${number(before.nsPerOp / 1000, 1)} microseconds before, ${number(after.nsPerOp / 1000, 1)} microseconds after">
    <h4>${entry.entries.toLocaleString('en-US')} entries<span>${number(speedup, 2)}× faster · ${number(allocationsReduced, 1)}% fewer allocs</span></h4>
    <div class="bar-row">
      <span class="bar-name">Before segment scan</span>
      <span class="bar-track" style="--tick-step:${tickStep}" aria-hidden="true"><span class="bar-fill baseline" style="width:${(before.nsPerOp / 1000 / max * 100).toFixed(2)}%"></span></span>
      <span class="bar-value">${number(before.nsPerOp / 1000, 1)} µs</span>
    </div>
    <div class="bar-row">
      <span class="bar-name">After segment scan</span>
      <span class="bar-track" style="--tick-step:${tickStep}" aria-hidden="true"><span class="bar-fill indexed" style="width:${(after.nsPerOp / 1000 / max * 100).toFixed(2)}%"></span></span>
      <span class="bar-value">${number(after.nsPerOp / 1000, 1)} µs</span>
    </div>
  </section>`).join('');
  const axis = `<div class="archive-lookup-axis" aria-hidden="true"><span></span><div class="axis-ticks">${tickMarkup(ticks)}</div><span></span><span class="axis-unit">Validation time (microseconds) · common 0–900 µs scale</span></div>`;
  document.querySelector('#zip-validation-chart').innerHTML = groups + axis;
  document.querySelector('#zip-validation-summary').textContent = `At 1,000 entries validation is ${number(rows[0].speedup, 2)}× faster; at 10,000 entries it is ${number(rows[1].speedup, 2)}× faster. The 10,000-entry case also uses ${number((1 - rows[1].after.bytesPerOp / rows[1].before.bytesPerOp) * 100, 1)}% fewer allocated bytes.`;
  document.querySelector('#zip-validation-table').innerHTML = `
    <thead><tr><th scope="col">Entries</th><th scope="col">Before</th><th scope="col">After</th><th scope="col">Bytes/op</th><th scope="col">Allocs/op</th></tr></thead>
    <tbody>${rows.map(({entry, before, after}) => `<tr><th scope="row">${entry.entries.toLocaleString('en-US')}</th><td>${number(before.nsPerOp / 1000, 1)} µs</td><td>${number(after.nsPerOp / 1000, 1)} µs</td><td>${number(before.bytesPerOp, 0)} → ${number(after.bytesPerOp, 0)}</td><td>${before.allocationsPerOp.toLocaleString('en-US')} → ${after.allocationsPerOp.toLocaleString('en-US')}</td></tr>`).join('')}</tbody>`;
}

function renderPage() {
  const { environment, measuredOn } = benchmarkData;
  document.querySelector('#environment').textContent = 'Standard password profile · disposable synthetic files';
  document.querySelector('#run-context').textContent = `Measured ${measuredOn}. CLI medians use ${environment.cliRepetitions} full runs; search and path checks use ${environment.goBenchmarkRepetitions} × ${environment.goBenchmarkBenchtimeMs} ms; V6 streams use ${benchmarkData.crypto.repetitions} × ${benchmarkData.crypto.benchmarkDurationMs} ms. RSS is sampled every ${environment.memorySampleIntervalMs} ms and may miss short peaks.`;
  renderStageControls();
  renderOperationChart();
  renderRssChart();
  renderCrypto();
  renderSearch();
  renderConflict();
  renderArchiveLookup();
  renderZipValidation();
}

workloadInputs.forEach((input) => input.addEventListener('change', () => {
  if (input.checked) {
    selectedWorkload = input.value;
    renderOperationChart();
  }
}));
stageSlider.addEventListener('input', () => setStage(Number(stageSlider.value)));

fetch('./benchmarks.json?v=20261108-row-update')
  .then((response) => {
    if (!response.ok) throw new Error(`Could not load benchmark data (${response.status}).`);
    return response.json();
  })
  .then((data) => {
    benchmarkData = data;
    renderPage();
  })
  .catch((error) => {
    const alert = document.createElement('p');
    alert.className = 'loading-message error-message';
    alert.setAttribute('role', 'alert');
    alert.textContent = `The benchmark page could not load its data: ${error.message}`;
    document.querySelector('main').prepend(alert);
  });
