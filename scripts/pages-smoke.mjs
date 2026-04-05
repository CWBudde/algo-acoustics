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
      console.log(`[response:${status}] ${response.request().method()} ${response.url()}`);
    }
  });

  page.on("pageerror", (error) => {
    console.log(`[pageerror] ${error.stack || error.message || String(error)}`);
    pageErrors.push(error.stack || error.message || String(error));
  });

  await waitForDeployedPage(page);
  log("demo reported ready");
  await page.waitForTimeout(2000);
  log("post-ready quiet period complete");

  await collectDiagnostics(page);
} finally {
  log(`finished after ${elapsedMs()}ms`);
  await browser.close();
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
        throw new Error(`unexpected HTTP status ${response?.status() ?? "unknown"}`);
      }

      log("waiting for window.algoAcousticsDemoReady");
      await page.waitForFunction(() => window.algoAcousticsDemoReady === true, undefined, {
        timeout: 120000,
      });
      log("window.algoAcousticsDemoReady observed");

      log("waiting for engine badge to show WASM ready");
      await page.waitForFunction(
        () => document.getElementById("engine-status")?.textContent?.trim() === "WASM ready",
        undefined,
        { timeout: 120000 },
      );
      log("engine badge ready");
      return;
    } catch (error) {
      console.log(`[attempt ${attempt}] failed: ${error?.stack || error?.message || String(error)}`);
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
      engineStatus: document.getElementById("engine-status")?.textContent?.trim() ?? null,
      renderBadge: document.getElementById("render-badge")?.textContent?.trim() ?? null,
      hasCanvas: Boolean(document.getElementById("scene-canvas")),
    }))
    .catch((error) => ({ error: error?.stack || error?.message || String(error) }));

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
