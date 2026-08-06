export class RenderWorkerController {
  constructor({ createWorker, onMessage = () => {}, onError = () => {} }) {
    this.createWorker = createWorker;
    this.onMessage = onMessage;
    this.onError = onError;
    this.worker = null;
    this.generation = 0;
    this.ready = false;
  }

  start() {
    return this.replace();
  }

  restart() {
    return this.replace();
  }

  postMessage(message) {
    if (!this.worker || !this.ready) {
      throw new Error("WASM worker is not ready");
    }

    this.worker.postMessage(message);
  }

  replace() {
    const generation = ++this.generation;
    this.ready = false;
    this.worker?.terminate();

    const worker = this.createWorker();
    this.worker = worker;

    const readyPromise = new Promise((resolve, reject) => {
      worker.addEventListener("message", (event) => {
        if (generation !== this.generation) {
          return;
        }

        if (event.data?.type === "ready") {
          this.ready = true;
          resolve();
          return;
        }

        if (!this.ready && event.data?.type === "error") {
          const error = new Error(event.data.message || "WASM worker failed");
          reject(error);
          this.onError(error);
          return;
        }

        this.onMessage(event);
      });

      worker.addEventListener("error", (event) => {
        if (generation !== this.generation) {
          return;
        }

        this.ready = false;
        const error =
          event.error ?? new Error(event.message || "WASM worker failed");
        reject(error);
        this.onError(error);
      });
    });

    worker.postMessage({ type: "init" });
    return readyPromise;
  }
}
