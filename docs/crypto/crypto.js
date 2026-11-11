const validationCases = [
  { entries: 1000, beforeUs: 71.4, afterUs: 49.4, beforeBytes: 86608, afterBytes: 54608, beforeAllocs: 1005, afterAllocs: 5 },
  { entries: 10000, beforeUs: 806.8, afterUs: 582.6, beforeBytes: 756866, afterBytes: 436865, beforeAllocs: 10033, afterAllocs: 33 },
];

const entrySlider = document.querySelector("#entry-slider");
if (entrySlider) {
  const elements = {
    count: document.querySelector("#entry-count"),
    afterBar: document.querySelector("#after-bar"),
    beforeTime: document.querySelector("#before-time"),
    afterTime: document.querySelector("#after-time"),
    speedup: document.querySelector("#speedup"),
    allocation: document.querySelector("#allocation-change"),
  };
  const updateValidationChart = () => {
    const result = validationCases[Number(entrySlider.value)];
    elements.count.value = result.entries.toLocaleString("en-US");
    elements.count.textContent = result.entries.toLocaleString("en-US");
    elements.count.setAttribute("aria-label", result.entries.toLocaleString("en-US") + " ZIP entries");
    elements.beforeTime.textContent = result.beforeUs.toLocaleString("en-US", { maximumFractionDigits: 1 }) + " µs";
    elements.afterTime.textContent = result.afterUs.toLocaleString("en-US", { maximumFractionDigits: 1 }) + " µs";
    elements.afterBar.style.width = (result.afterUs / result.beforeUs * 100).toFixed(1) + "%";
    elements.speedup.textContent = (result.beforeUs / result.afterUs).toFixed(2) + "× faster";
    elements.allocation.textContent = result.beforeAllocs.toLocaleString("en-US") + " → " +
      result.afterAllocs.toLocaleString("en-US") + " allocations/op · " +
      result.beforeBytes.toLocaleString("en-US") + " → " + result.afterBytes.toLocaleString("en-US") + " B/op";
    entrySlider.setAttribute("aria-valuetext", result.entries.toLocaleString("en-US") + " synthetic ZIP entries");
  };
  entrySlider.addEventListener("input", updateValidationChart);
  updateValidationChart();
}

const encryptButton = document.querySelector("#encrypt-button");
if (encryptButton) {
  const decryptButton = document.querySelector("#decrypt-button");
  const resetButton = document.querySelector("#reset-button");
  const status = document.querySelector("#demo-status");
  const result = document.querySelector("#demo-result");
  const cipherOutput = document.querySelector("#cipher-output");
  const cipherSize = document.querySelector("#cipher-size");
  const decryptOutput = document.querySelector("#decrypt-output");
  const steps = [...document.querySelectorAll(".trace-step")];
  let worker;
  let requestId = 0;
  let busy = false;
  let hasCiphertext = false;

  const setStep = (name, state) => {
    const step = steps.find((entry) => entry.dataset.stage === name);
    if (!step) return;
    step.dataset.state = state;
    const label = step.querySelector(".trace-state");
    label.textContent = state === "active" ? "Working" : state === "done" ? "Complete" : "Waiting";
    if (state === "active") step.setAttribute("aria-current", "step");
    else step.removeAttribute("aria-current");
  };

  const resetTrace = () => steps.forEach((step) => {
    delete step.dataset.state;
    step.removeAttribute("aria-current");
    step.querySelector(".trace-state").textContent = "Waiting";
  });

  const lockControls = (locked) => {
    busy = locked;
    encryptButton.disabled = locked || !worker;
    encryptButton.textContent = locked ? "Working in the local worker…" : "Click to encrypt this sample";
    decryptButton.disabled = locked || !hasCiphertext;
    resetButton.disabled = locked || !hasCiphertext;
  };

  const showFatal = (message) => {
    worker?.terminate();
    worker = null;
    encryptButton.disabled = true;
    encryptButton.textContent = "Demo unavailable";
    status.textContent = "The local crypto demo could not start: " + message;
  };

  try {
    worker = new Worker("./worker.js", { name: "pulse-vault-crypto-demo" });
    worker.onmessage = ({ data }) => {
      if (data.type === "fatal") {
        showFatal(data.message);
        return;
      }
      if (data.type === "ready") {
        encryptButton.disabled = false;
        encryptButton.textContent = "Click to encrypt this sample";
        status.textContent = "Ready. The Go crypto module is running locally in a browser worker.";
        return;
      }
      if (data.id !== requestId) return;
      if (data.type === "progress") {
        if (data.stage === "kdf") {
          resetTrace();
          setStep("kdf", "active");
          status.textContent = "Deriving the standard-profile demo key in the worker…";
        } else if (data.stage === "stream") {
          setStep("kdf", "done");
          setStep("split", "done");
          setStep("stream", "active");
          setStep("aead", "active");
          status.textContent = "Writing and authenticating V6 stream records…";
        }
        return;
      }
      if (data.type === "encrypted") {
        ["stream", "aead", "terminal"].forEach((name) => setStep(name, "done"));
        hasCiphertext = true;
        cipherOutput.textContent = data.ciphertext;
        cipherSize.textContent = data.format + " · " + data.cipherBytes.toLocaleString("en-US") +
          " bytes · salt " + data.salt;
        decryptOutput.hidden = true;
        result.hidden = false;
        status.textContent = "Encrypted " + data.plaintextBytes +
          " synthetic bytes using the public demo key. The ciphertext is disposable.";
        lockControls(false);
        return;
      }
      if (data.type === "decrypted") {
        ["kdf", "split", "stream", "aead", "terminal"].forEach((name) => setStep(name, "done"));
        decryptOutput.textContent = "Recovered sample: " + data.plaintext;
        decryptOutput.hidden = false;
        status.textContent = "The sample decrypted and authenticated successfully in this worker.";
        lockControls(false);
        return;
      }
      if (data.type === "reset") {
        hasCiphertext = false;
        result.hidden = true;
        cipherOutput.textContent = "";
        decryptOutput.textContent = "";
        decryptOutput.hidden = true;
        resetTrace();
        status.textContent = "Demo state cleared. Encrypt a fresh sample when ready.";
        lockControls(false);
        return;
      }
      if (data.type === "error") {
        status.textContent = data.message;
        lockControls(false);
      }
    };
    worker.onerror = (event) => showFatal(event.message || "worker error");
  } catch (error) {
    showFatal(String(error));
  }

  encryptButton.addEventListener("click", () => {
    if (!worker || busy) return;
    result.hidden = true;
    decryptOutput.hidden = true;
    hasCiphertext = false;
    resetTrace();
    setStep("kdf", "active");
    requestId += 1;
    lockControls(true);
    status.textContent = "Deriving the standard-profile demo key in the worker…";
    worker.postMessage({ id: requestId, op: "encrypt" });
  });

  decryptButton.addEventListener("click", () => {
    if (!worker || busy || !hasCiphertext) return;
    requestId += 1;
    lockControls(true);
    status.textContent = "Checking and decrypting the stream in the worker…";
    setStep("aead", "active");
    worker.postMessage({ id: requestId, op: "decrypt" });
  });

  resetButton.addEventListener("click", () => {
    if (!worker || busy) return;
    requestId += 1;
    lockControls(true);
    worker.postMessage({ id: requestId, op: "reset" });
  });
}
