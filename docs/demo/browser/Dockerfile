# Browser recorder: Playwright's own image (Chromium and its dependencies at
# the same version as the npm package), with this directory's lockfile.
# The same image runs lib/gw.mjs, the loopback forwarder, and renders the
# title and end cards from lib/card.html.
FROM mcr.microsoft.com/playwright:v1.55.0-noble@sha256:7e20a8225950eb93a9d306a917b05d822a68a67333973e254f1b0cd86c0f3742
# node_modules lives at /work so that ES-module imports from /work/scene and /work/lib resolve it
WORKDIR /work
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
