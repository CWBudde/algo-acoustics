import { chromium } from "playwright";

const pageUrl = process.argv[2] ?? process.env.PAGE_URL;

if (!pageUrl) {
  throw new Error("Missing GitHub Pages URL");
}

const consoleErrors = [];
const pageErrors = [];
const requestFailures = [];
const startedAt = Date.now();

log(`starting smoke test for ${pageUrl}`);
const browser = await chromium.launch({ headless: true });
log("browser launched");

try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 960 },
  });
  log("page created");
  await page.addInitScript(() => {
    const startedAt = performance.now();
    const stamp = () => `+[${Math.round(performance.now() - startedAt)}ms]`;
    const originalWorker = window.Worker;
    window.__pagesSmoke = {
      workerEvents: [],
      fetches: [],
      windowErrors: [],
    };

    window.addEventListener("error", (event) => {
      const text = `${stamp()} [window.error] ${event.message} @ ${event.filename}:${event.lineno}:${event.colno}`;
      console.log(text);
      window.__pagesSmoke.windowErrors.push(text);
    });

    window.addEventListener("unhandledrejection", (event) => {
      const text = `${stamp()} [window.unhandledrejection] ${String(event.reason)}`;
      console.log(text);
      window.__pagesSmoke.windowErrors.push(text);
    });

    window.Worker = new Proxy(originalWorker, {
      construct(target, args, newTarget) {
        const [scriptUrl, options] = args;
        const text =
          `${stamp()} [worker.construct] ${String(scriptUrl)} ${options ? JSON.stringify(options) : ""}`.trim();
        console.log(text);
        const worker = Reflect.construct(target, args, newTarget);
        worker.addEventListener("message", (event) => {
          const payload =
            typeof event.data === "string"
              ? event.data
              : JSON.stringify(event.data);
          const message = `${stamp()} [worker.message] ${payload}`;
          console.log(message);
          window.__pagesSmoke.workerEvents.push(message);
        });
        worker.addEventListener("error", (event) => {
          const text = `${stamp()} [worker.error] ${event.message} @ ${event.filename}:${event.lineno}:${event.colno}`;
          console.log(text);
          window.__pagesSmoke.workerEvents.push(text);
        });
        return worker;
      },
    });

    const originalFetch = window.fetch.bind(window);
    window.fetch = async (...args) => {
      const [input] = args;
      const url =
        typeof input === "string" ? input : (input?.url ?? String(input));
      const text = `${stamp()} [fetch] ${url}`;
      console.log(text);
      window.__pagesSmoke.fetches.push(text);
      try {
        const response = await originalFetch(...args);
        console.log(`${stamp()} [fetch.response] ${response.status} ${url}`);
        return response;
      } catch (error) {
        console.log(
          `${stamp()} [fetch.error] ${url} ${error?.stack || error?.message || String(error)}`,
        );
        throw error;
      }
    };
  });

  page.on("console", (message) => {
    const text = `[console:${message.type()}] ${message.text()}`;
    console.log(text);
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });

  page.on("requestfailed", (request) => {
    const failure = request.failure();
    const text = `[requestfailed] ${request.method()} ${request.url()} ${failure?.errorText ?? "unknown error"}`;
    console.log(text);
    requestFailures.push(text);
  });

  page.on("response", (response) => {
    const status = response.status();
    if (status >= 400) {
      console.log(
        `[response:${status}] ${response.request().method()} ${response.url()}`,
      );
    }
  });

  page.on("pageerror", (error) => {
    console.log(`[pageerror] ${error.stack || error.message || String(error)}`);
    pageErrors.push(error.stack || error.message || String(error));
  });

  await waitForDeployedPage(page);
  log("demo reported ready");
  await assertWasmDelivery(page);
  log("wasm delivery headers verified");
  await renderImpulseResponse(page);
  log("demo render completed");
  await auditionAndUpdateImpulseResponse(page);
  log("demo auralization and IR update completed");

  await collectDiagnostics(page);
} finally {
  log(`finished after ${elapsedMs()}ms`);
  await browser.close();
}

// assertWasmDelivery checks the headers the host sends for the demo binary.
// WebAssembly.instantiateStreaming, which web/worker.js uses, rejects any
// response that is not typed application/wasm, so a host misconfiguration must
// fail the deployment smoke test rather than silently degrade the demo.
async function assertWasmDelivery(page) {
  const wasmUrl = new URL("algo_acoustics_demo.wasm", pageUrl).toString();
  const response = await page.request.get(wasmUrl);

  if (!response.ok()) {
    throw new Error(`${wasmUrl} returned HTTP ${response.status()}`);
  }

  const headers = response.headers();
  log(
    `wasm headers: content-type=${headers["content-type"] ?? "(none)"} cache-control=${
      headers["cache-control"] ?? "(none)"
    } etag=${headers["etag"] ?? "(none)"}`,
  );

  const contentType = (headers["content-type"] ?? "").split(";")[0].trim();
  if (contentType !== "application/wasm") {
    throw new Error(
      `${wasmUrl} served as "${contentType || "(none)"}", want "application/wasm"; ` +
        "WebAssembly.instantiateStreaming rejects any other type",
    );
  }

  // An unhashed filename must stay revalidatable, otherwise a deployed update
  // cannot reach browsers that already cached the previous build.
  if (!headers["etag"] && !headers["last-modified"]) {
    throw new Error(
      `${wasmUrl} sent neither ETag nor Last-Modified, so caches cannot revalidate it`,
    );
  }
}

async function renderImpulseResponse(page) {
  await page.evaluate(() => {
    const setRange = (id, value) => {
      const input = document.getElementById(id);
      if (!input) {
        throw new Error(`missing range input #${id}`);
      }
      input.value = String(value);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    };

    setRange("render-rays", 128);
    setRange("render-order", 1);
    setRange("render-duration", 0.25);
    const earlyMode = document.querySelector('[data-mode="early"]');
    const renderButton = document.getElementById("render-scene");
    if (!earlyMode || !renderButton) {
      throw new Error("missing render controls");
    }
    earlyMode.click();
    renderButton.click();
  });

  await page.waitForFunction(
    () => {
      const result = window.algoAcousticsDemoLastRender;
      return (
        document.getElementById("render-badge")?.textContent?.trim() ===
          "Render complete" &&
        result?.sampleCount > 0 &&
        result?.wavByteLength > 44 &&
        document.getElementById("spl-heatmap")?.getAttribute("aria-pressed") ===
          "true" &&
        document.getElementById("spl-heatmap")?.disabled === false &&
        document.getElementById("spl-heatmap-legend")?.hidden === false
      );
    },
    undefined,
    { timeout: 120000 },
  );
}

async function auditionAndUpdateImpulseResponse(page) {
  const previousRequestID = await page.evaluate(
    () => window.algoAcousticsDemoLastRender?.requestId ?? 0,
  );
  await page.evaluate(() => {
    const drySource = document.getElementById("dry-source");
    const playButton = document.getElementById("play-auralization");
    if (!drySource || !playButton) {
      throw new Error("missing auralization controls");
    }

    drySource.value = "music";
    drySource.dispatchEvent(new Event("change", { bubbles: true }));
    playButton.click();
  });

  await page.waitForFunction(
    () =>
      document.getElementById("audio-status")?.textContent?.trim() ===
      "Playing",
    undefined,
    { timeout: 30000 },
  );

  await page.evaluate(() => {
    const receiverX = document.getElementById("receiver-x");
    const renderButton = document.getElementById("render-scene");
    const audioStatus = document.getElementById("audio-status");
    if (!receiverX || !renderButton || !audioStatus) {
      throw new Error("missing render controls");
    }
    window.__pagesSmoke.sawIRTransition = false;
    const observer = new MutationObserver(() => {
      if (audioStatus.textContent?.trim() === "Updating IR") {
        window.__pagesSmoke.sawIRTransition = true;
        observer.disconnect();
      }
    });
    observer.observe(audioStatus, { childList: true, subtree: true });
    receiverX.value = String(Number(receiverX.value) - 0.1);
    receiverX.dispatchEvent(new Event("input", { bubbles: true }));
    renderButton.click();
  });

  await page.waitForFunction(
    (requestID) =>
      (window.algoAcousticsDemoLastRender?.requestId ?? 0) > requestID &&
      document.getElementById("audio-status")?.textContent?.trim() ===
        "Playing",
    previousRequestID,
    { timeout: 120000 },
  );

  const audioResult = await page.evaluate(() => ({
    fetchedMusic: window.__pagesSmoke.fetches.some((entry) =>
      entry.includes("audio/music.mp3"),
    ),
    sawIRTransition: window.__pagesSmoke.sawIRTransition,
  }));
  if (!audioResult.fetchedMusic) {
    throw new Error("bundled music sample was not fetched");
  }
  if (!audioResult.sawIRTransition) {
    throw new Error("active playback did not transition to the updated IR");
  }
}

async function waitForDeployedPage(page) {
  const maxAttempts = 10;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      log(`navigation attempt ${attempt}/${maxAttempts}`);
      const response = await page.goto(pageUrl, {
        waitUntil: "domcontentloaded",
        timeout: 120000,
      });
      log(
        `goto returned status ${response?.status() ?? "unknown"}, current url ${page.url()}`,
      );

      if (!response || response.status() >= 400) {
        throw new Error(
          `unexpected HTTP status ${response?.status() ?? "unknown"}`,
        );
      }

      log("waiting for window.algoAcousticsDemoReady");
      await page.waitForFunction(
        () => window.algoAcousticsDemoReady === true,
        undefined,
        {
          timeout: 120000,
        },
      );
      log("window.algoAcousticsDemoReady observed");

      log("waiting for engine badge to show WASM ready");
      await page.waitForFunction(
        () =>
          document.getElementById("engine-status")?.textContent?.trim() ===
          "WASM ready",
        undefined,
        { timeout: 120000 },
      );
      log("engine badge ready");
      return;
    } catch (error) {
      console.log(
        `[attempt ${attempt}] failed: ${error?.stack || error?.message || String(error)}`,
      );
      if (attempt === maxAttempts) {
        throw error;
      }

      log(`retrying after delay`);
      await page.waitForTimeout(15000);
    }
  }
}

async function collectDiagnostics(page) {
  const title = await page.title().catch(() => "");
  const url = page.url();
  const ready = await page
    .evaluate(() => ({
      ready: window.algoAcousticsDemoReady === true,
      engineStatus:
        document.getElementById("engine-status")?.textContent?.trim() ?? null,
      renderBadge:
        document.getElementById("render-badge")?.textContent?.trim() ?? null,
      lastRender: window.algoAcousticsDemoLastRender ?? null,
      hasCanvas: Boolean(document.getElementById("scene-canvas")),
      smoke: window.__pagesSmoke ?? null,
    }))
    .catch((error) => ({
      error: error?.stack || error?.message || String(error),
    }));

  log(`page title: ${title || "(empty)"}`);
  log(`final url: ${url}`);
  log(`page state: ${JSON.stringify(ready)}`);

  if (requestFailures.length) {
    log(`request failures: ${JSON.stringify(requestFailures)}`);
  }

  if (consoleErrors.length || pageErrors.length) {
    const details = [];
    if (consoleErrors.length) {
      details.push(`console errors:\n- ${consoleErrors.join("\n- ")}`);
    }
    if (pageErrors.length) {
      details.push(`page errors:\n- ${pageErrors.join("\n- ")}`);
    }
    throw new Error(details.join("\n"));
  }
}

function log(message) {
  console.log(`[pages-smoke +${elapsedMs()}ms] ${message}`);
}

function elapsedMs() {
  return Date.now() - startedAt;
}
