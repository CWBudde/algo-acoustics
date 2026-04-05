import { chromium } from "playwright";

const pageUrl = process.argv[2] ?? process.env.PAGE_URL;

if (!pageUrl) {
  throw new Error("Missing GitHub Pages URL");
}

const consoleErrors = [];
const pageErrors = [];

const browser = await chromium.launch({ headless: true });

try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 960 },
  });

  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });

  page.on("pageerror", (error) => {
    pageErrors.push(error.stack || error.message || String(error));
  });

  await waitForDeployedPage(page);
  await page.waitForTimeout(2000);

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
} finally {
  await browser.close();
}

async function waitForDeployedPage(page) {
  const maxAttempts = 10;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      const response = await page.goto(pageUrl, {
        waitUntil: "domcontentloaded",
        timeout: 120000,
      });

      if (!response || response.status() >= 400) {
        throw new Error(`unexpected HTTP status ${response?.status() ?? "unknown"}`);
      }

      await page.waitForFunction(() => window.algoAcousticsDemoReady === true, undefined, {
        timeout: 120000,
      });
      await page.waitForFunction(
        () => document.getElementById("engine-status")?.textContent?.trim() === "WASM ready",
        undefined,
        { timeout: 120000 },
      );
      return;
    } catch (error) {
      if (attempt === maxAttempts) {
        throw error;
      }

      await page.waitForTimeout(15000);
    }
  }
}
