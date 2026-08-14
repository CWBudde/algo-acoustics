# Web Demo Deployment

How to serve the browser demo in `web/` so that the Go/WASM module loads. The
build itself is covered in [web/README.md](../web/README.md); this page is about
the HTTP layer.

## The one hard requirement: `application/wasm`

`web/worker.js` instantiates the module with
`WebAssembly.instantiateStreaming(fetch("algo_acoustics_demo.wasm"), …)`.
Streaming compilation is specified to reject any response whose `Content-Type`
is not exactly `application/wasm`; the browser reports a `TypeError` mentioning
an incorrect response MIME type and the demo never reaches "WASM ready".

A host that types the binary as `application/octet-stream` or `text/plain`
therefore breaks the demo outright. This is the single header that must be
correct.

## Caching

`algo_acoustics_demo.wasm` has no content hash in its name, so the same URL
serves a different binary after every deployment. Immutable long-lived caching
(`max-age=31536000, immutable`) would pin returning visitors to a stale build,
so the demo uses revalidation instead:

| Asset                              | `Cache-Control`         | Rationale                                                   |
| ---------------------------------- | ----------------------- | ----------------------------------------------------------- |
| `algo_acoustics_demo.wasm`         | `no-cache`              | Unhashed name; must revalidate so a new build is picked up. |
| `*.js`, `*.mjs`, `*.css`, `*.html` | `no-cache`              | Same reason.                                                |
| `audio/*.mp3`                      | `public, max-age=86400` | Bundled dry signals; only replaced wholesale.               |

`no-cache` does not mean "do not store". The browser keeps the entry and sends a
conditional request; the host answers `304 Not Modified` from the `ETag` or
`Last-Modified` validator, so the multi-megabyte binary is re-sent only when it
actually changed. Any host serving the demo must therefore emit at least one of
those validators — the deployment smoke test in `scripts/pages-smoke.mjs`
enforces both this and the MIME type against the live site.

## Cross-origin isolation (COOP/COEP)

**Not enabled, and not needed.** `SharedArrayBuffer` is only available to a
cross-origin-isolated document, which requires
`Cross-Origin-Opener-Policy: same-origin` plus
`Cross-Origin-Embedder-Policy: require-corp`. The demo does not use it: rendering
runs in a plain dedicated `Worker` (`web/worker.js`), and results cross the
worker boundary through structured-clone `postMessage`, so no shared memory is
involved. Go's `GOOS=js` runtime is single-threaded and does not use
`SharedArrayBuffer` either.

Enabling `require-corp` would not be free. `web/index.html` loads three.js from
`cdn.jsdelivr.net` and fonts from Google Fonts; under COEP every cross-origin
subresource must opt in via CORS or `Cross-Origin-Resource-Policy`, so the policy
adds a third-party dependency for no gain.

If a future change does need `SharedArrayBuffer` (e.g. threaded WASM):

1. Send both headers on the **document** response, then confirm
   `window.crossOriginIsolated === true`.
2. Self-host or CORS-verify three.js and the fonts, or accept them being blocked.
3. On GitHub Pages, which cannot send custom headers (see below), the usual
   workaround is a `coi-serviceworker` shim that re-serves responses with the
   headers attached — a fallback, not an equivalent.

The local dev server implements the headers behind a flag so the policy can be
tested before committing to it:

```bash
go run ./web/devserver -dir web -coi
```

## Local development

```bash
./web/build-wasm.sh
just web-demo          # go run ./web/devserver -dir web -addr :8080
```

Do not use `python3 -m http.server` for the demo. Whether it types `.wasm`
correctly depends on the host's `/etc/mime.types`, and on systems without a wasm
entry it falls back to `application/octet-stream`, which fails streaming
compilation. `web/devserver` sets the MIME types from its own table, applies the
cache policy above, and emits an `ETag` derived from file size and mtime.

## Host configuration

### GitHub Pages (the deployed demo)

`.github/workflows/pages.yml` uploads `web/` and deploys it. GitHub Pages serves
`.wasm` as `application/wasm` and attaches an `ETag` to every asset, so the demo
works without configuration. It does **not** accept custom headers — there is no
`_headers`, `.htaccess`, or config hook — so its own defaults apply. As of the
current deployment those are `content-type: application/wasm`,
`cache-control: max-age=600`, plus `ETag` and `Last-Modified`. That is compatible with the policy
above: a new deployment is picked up within ten minutes at worst, and
conditional requests keep the binary from being re-downloaded in the meantime.

`.github/workflows/pages-smoke.yml` runs against the live URL after each deploy
and fails if the MIME type or the cache validator regresses.

### Netlify / Cloudflare Pages

`web/_headers` is already in the deployed tree and both hosts read it as-is.

### nginx

```nginx
types {
    application/wasm  wasm;
}

location ~* \.(wasm|js|mjs|css|html)$ {
    add_header Cache-Control "no-cache";
}

location /audio/ {
    add_header Cache-Control "public, max-age=86400";
}
```

Recent `nginx` releases ship a `wasm` entry in `mime.types`; the explicit `types`
block above is only needed on older builds. Check with
`grep wasm /etc/nginx/mime.types`.

### Apache

```apache
AddType application/wasm .wasm

<FilesMatch "\.(wasm|js|mjs|css|html)$">
    Header set Cache-Control "no-cache"
</FilesMatch>

<LocationMatch "^/audio/">
    Header set Cache-Control "public, max-age=86400"
</LocationMatch>
```

### Caddy

Caddy types `.wasm` correctly out of the box:

```caddyfile
example.com {
    root * /srv/algo-acoustics-demo
    file_server

    @revalidate path *.wasm *.js *.mjs *.css *.html
    header @revalidate Cache-Control "no-cache"

    @audio path /audio/*
    header @audio Cache-Control "public, max-age=86400"
}
```

## Verifying a deployment by hand

```bash
curl -sSI https://<host>/algo_acoustics_demo.wasm | grep -i -E 'content-type|cache-control|etag|last-modified'
```

Expect `content-type: application/wasm` and at least one of `etag` or
`last-modified`.
