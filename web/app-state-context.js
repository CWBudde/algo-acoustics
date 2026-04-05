let ctx = null;

export function setAppStateContext(nextCtx) {
  ctx = nextCtx;
}

export function requireAppStateContext() {
  if (!ctx) {
    throw new Error("App state context not initialized");
  }
  return ctx;
}
