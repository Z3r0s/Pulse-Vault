importScripts("./wasm_exec.js");

const go = new Go();
WebAssembly.instantiateStreaming(fetch("./pulse-vault-crypto.wasm"), go.importObject)
  .then(({ instance }) => go.run(instance))
  .catch((error) => postMessage({ type: "fatal", message: String(error) }));
