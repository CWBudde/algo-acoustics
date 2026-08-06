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
      postMessage({ type: "ready" });
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
