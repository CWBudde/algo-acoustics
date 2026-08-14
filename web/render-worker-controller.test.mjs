import test from "node:test";
import assert from "node:assert/strict";

import { RenderWorkerController } from "./render-worker-controller.mjs";

class FakeWorker {
  constructor() {
    this.listeners = new Map();
    this.messages = [];
    this.terminated = false;
  }

  addEventListener(type, listener) {
    this.listeners.set(type, listener);
  }

  postMessage(message) {
    this.messages.push(message);
  }

  terminate() {
    this.terminated = true;
  }

  emit(type, payload) {
    this.listeners.get(type)?.(payload);
  }
}

test("controller waits for worker readiness before accepting renders", async () => {
  const worker = new FakeWorker();
  const controller = new RenderWorkerController({ createWorker: () => worker });

  const started = controller.start();
  assert.deepEqual(worker.messages, [{ type: "init" }]);
  assert.throws(() => controller.postMessage({ type: "render" }), /not ready/);

  worker.emit("message", { data: { type: "ready" } });
  await started;

  controller.postMessage({ type: "render", requestId: 1 });
  assert.deepEqual(worker.messages.at(-1), { type: "render", requestId: 1 });
});

test("restart terminates a synchronous render worker and ignores its stale result", async () => {
  const workers = [];
  const messages = [];
  const controller = new RenderWorkerController({
    createWorker: () => {
      const worker = new FakeWorker();
      workers.push(worker);
      return worker;
    },
    onMessage: (event) => messages.push(event.data),
  });

  const firstStart = controller.start();
  workers[0].emit("message", { data: { type: "ready" } });
  await firstStart;
  controller.postMessage({ type: "render", requestId: 1 });

  const restarted = controller.restart();
  assert.equal(workers[0].terminated, true);
  assert.equal(controller.ready, false);
  assert.deepEqual(workers[1].messages, [{ type: "init" }]);

  workers[0].emit("message", {
    data: { type: "result", requestId: 1, result: { samples: [1] } },
  });
  assert.deepEqual(messages, []);

  workers[1].emit("message", { data: { type: "ready" } });
  await restarted;
  controller.postMessage({ type: "render", requestId: 2 });

  assert.deepEqual(workers[1].messages.at(-1), {
    type: "render",
    requestId: 2,
  });
});

test("controller rejects initialization errors reported as worker messages", async () => {
  const worker = new FakeWorker();
  const errors = [];
  const controller = new RenderWorkerController({
    createWorker: () => worker,
    onError: (error) => errors.push(error.message),
  });

  const started = controller.start();
  worker.emit("message", {
    data: { type: "error", message: "failed to instantiate WASM" },
  });

  await assert.rejects(started, /failed to instantiate WASM/);
  assert.deepEqual(errors, ["failed to instantiate WASM"]);
  assert.equal(controller.ready, false);
});

test("memory budget fields on a result reach the message handler intact", async () => {
  const worker = new FakeWorker();
  const received = [];
  const controller = new RenderWorkerController({
    createWorker: () => worker,
    onMessage: (event) => received.push(event.data),
  });

  const started = controller.start();
  worker.emit("message", { data: { type: "ready" } });
  await started;

  // The Go side attaches `memory` and `warnings` to every render result; they
  // must survive the worker boundary so the page can report a downgrade.
  const result = {
    mode: "hybrid",
    warnings: ["memory budget: connected-room render reduced rays from 16384 to 4096"],
    memory: {
      heapBytes: 3_200_000,
      sysBytes: 128_000_000,
      peakSysBytes: 141_000_000,
      budgetBytes: 536_870_912,
      estimateBytes: 210_000_000,
    },
  };
  worker.emit("message", { data: { type: "result", requestId: 7, result } });

  assert.deepEqual(received.at(-1).result.warnings, result.warnings);
  assert.deepEqual(received.at(-1).result.memory, result.memory);
});
