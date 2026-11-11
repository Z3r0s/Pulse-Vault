#!/usr/bin/env node
"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const { webcrypto } = require("node:crypto");

const [wasmPath, wasmExecPath] = process.argv.slice(2);
if (!wasmPath || !wasmExecPath) {
  console.error("Usage: node scripts/test-crypto-wasm-demo.cjs <demo.wasm> <wasm_exec.js>");
  process.exit(2);
}

if (!globalThis.crypto) {
  Object.defineProperty(globalThis, "crypto", { value: webcrypto, configurable: true });
}
require(wasmExecPath);
assert.equal(typeof globalThis.Go, "function", "Go WebAssembly runtime did not load");

const expectedPlaintext =
  "Research starts with a question. An authenticated stream keeps these sample bytes private until each record passes its integrity check.";
const encryptID = 71;
const decryptID = 72;
const progressStages = [];
let state = "starting";
let finished = false;

const timeout = setTimeout(() => fail(new Error("demo did not finish within 30 seconds")), 30_000);

function fail(error) {
  if (finished) return;
  finished = true;
  clearTimeout(timeout);
  console.error("Go/WebAssembly crypto demo round trip failed:", error.message || String(error));
  process.exit(1);
}

function sendOperation(id, op) {
  assert.equal(typeof globalThis.onmessage, "function", "demo worker did not register its message handler");
  globalThis.onmessage({ data: { id, op } });
}

globalThis.postMessage = (message) => {
  if (finished) return;

  try {
    if (message.type === "fatal" || message.type === "error") {
      throw new Error(message.message || message.type);
    }

    if (message.type === "ready") {
      assert.equal(state, "starting");
      state = "encrypting";
      setImmediate(() => sendOperation(encryptID, "encrypt"));
      return;
    }

    if (message.type === "progress") {
      assert.equal(message.id, encryptID);
      progressStages.push(message.stage);
      return;
    }

    if (message.type === "encrypted") {
      assert.equal(state, "encrypting");
      assert.equal(message.id, encryptID);
      assert.deepEqual(progressStages, ["kdf", "stream"]);
      assert.equal(message.format, "PV6STRM1");
      assert.ok(message.cipherBytes > 0);
      assert.ok(message.salt.length > 0);
      assert.ok(message.ciphertext.length > 0);
      assert.equal(message.plaintextBytes, Buffer.byteLength(expectedPlaintext));
      state = "decrypting";
      setImmediate(() => sendOperation(decryptID, "decrypt"));
      return;
    }

    if (message.type === "decrypted") {
      assert.equal(state, "decrypting");
      assert.equal(message.id, decryptID);
      assert.equal(message.plaintext, expectedPlaintext);
      finished = true;
      clearTimeout(timeout);
      console.log("Go/WebAssembly crypto demo encrypt/decrypt round trip passed");
      process.exit(0);
    }
  } catch (error) {
    fail(error);
  }
};

const go = new globalThis.Go();
WebAssembly.instantiate(fs.readFileSync(wasmPath), go.importObject)
  .then(({ instance }) => go.run(instance))
  .catch(fail);
