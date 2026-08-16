/* eslint-env worker */

let wasmReady = false;
let currentRequestId = 0;
let goRuntime = null;

// Rendering is synchronous inside Go/WASM. The page cancels it by terminating
// this worker and creating a fresh one, because a cancel message cannot run
// until the render has already returned.
self.addEventListener("message", async (event) => {
  const message = event.data ?? {};

  if (message.type === "init") {
    try {
      await initWasm();
      // The request envelope is defined once, in Go (web/wasm/limits.go). It
      // travels with the ready message so the page sizes its sliders from the
      // limits that are actually enforced instead of repeating them in HTML.
      postMessage({
        type: "ready",
        limits: self.algoAcousticsDemo?.limits ?? null,
      });
    } catch (error) {
      postMessage({ type: "error", message: String(error) });
    }
    return;
  }

  if (message.type === "render") {
    if (!wasmReady || !self.algoAcousticsDemo?.renderScene) {
      postMessage({
        type: "error",
        requestId: message.requestId,
        message: "WASM runtime is not ready",
      });
      return;
    }

    currentRequestId = message.requestId;
    self.algoAcousticsDemoProgress = (stage, percent, detail) => {
      postMessage({
        type: "progress",
        requestId: currentRequestId,
        stage,
        percent,
        message: detail,
      });
    };

    // Progressive tiers (docs/web-demo-limits.md). Go calls this from inside
    // the render, which is synchronous and holds this worker for its whole
    // duration.
    // postMessage still delivers, because it queues the message on the page's
    // thread rather than needing this worker's own event loop.
    self.algoAcousticsDemoTier = (tier, payload) => {
      postMessage({
        type: "tier",
        requestId: currentRequestId,
        tier,
        payload,
      });
    };

    try {
      const result = self.algoAcousticsDemo.renderScene(message.payload ?? {});
      if (result?.error) {
        postMessage({
          type: "error",
          requestId: currentRequestId,
          message: result.error,
        });
        return;
      }

      postMessage({
        type: "result",
        requestId: currentRequestId,
        result,
      });
    } catch (error) {
      postMessage({
        type: "error",
        requestId: currentRequestId,
        message: String(error),
      });
    } finally {
      delete self.algoAcousticsDemoProgress;
      delete self.algoAcousticsDemoTier;
    }
  }
});

async function initWasm() {
  if (wasmReady) {
    return;
  }

  importScripts("wasm_exec.js");
  goRuntime = new Go();

  const response = await WebAssembly.instantiateStreaming(
    fetch("algo_acoustics_demo.wasm"),
    goRuntime.importObject,
  );
  goRuntime.run(response.instance);

  wasmReady = true;
}
