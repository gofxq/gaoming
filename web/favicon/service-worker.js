const CACHE_VERSION = "v2";
const SHELL_CACHE = `gaoming-shell-${CACHE_VERSION}`;
const STATIC_CACHE = `gaoming-static-${CACHE_VERSION}`;
const API_CACHE = `gaoming-api-${CACHE_VERSION}`;

const PRECACHE_URLS = [
  "/",
  "/index.html",
  "/site.webmanifest",
  "/favicon.ico",
  "/favicon-16x16.png",
  "/favicon-32x32.png",
  "/apple-touch-icon.png",
  "/android-chrome-192x192.png",
  "/android-chrome-512x512.png",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE).then((cache) => cache.addAll(PRECACHE_URLS)),
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  const validCaches = new Set([SHELL_CACHE, STATIC_CACHE, API_CACHE]);
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => !validCaches.has(key))
          .map((key) => caches.delete(key)),
      ),
    ),
  );
  self.clients.claim();
});

function isSuccessful(response) {
  return response && response.ok;
}

async function putInCache(cacheName, request, response) {
  if (!isSuccessful(response)) {
    return response;
  }

  const cache = await caches.open(cacheName);
  await cache.put(request, response.clone());
  return response;
}

async function cacheFirst(request, cacheName) {
  const cached = await caches.match(request);
  if (cached) {
    return cached;
  }

  const response = await fetch(request);
  return putInCache(cacheName, request, response);
}

async function staleWhileRevalidate(request, cacheName) {
  const cached = await caches.match(request);
  const networkFetch = fetch(request)
    .then((response) => putInCache(cacheName, request, response))
    .catch(() => undefined);

  if (cached) {
    void networkFetch;
    return cached;
  }

  const response = await networkFetch;
  if (response) {
    return response;
  }

  throw new Error("Network unavailable and no cached response");
}

async function networkFirst(request, cacheName, fallbackPath) {
  try {
    const response = await fetch(request);
    return await putInCache(cacheName, request, response);
  } catch {
    const cached = await caches.match(request);
    if (cached) {
      return cached;
    }

    if (fallbackPath) {
      const fallback = await caches.match(fallbackPath);
      if (fallback) {
        return fallback;
      }
    }

    throw new Error("Network unavailable and no cached fallback");
  }
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);

  if (request.method !== "GET") {
    return;
  }

  if (url.origin !== self.location.origin) {
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(networkFirst(request, SHELL_CACHE, "/index.html"));
    return;
  }

  if (url.pathname.startsWith("/master/api/v1/stream/")) {
    return;
  }

  if (url.pathname.startsWith("/master/api/v1/hosts")) {
    event.respondWith(networkFirst(request, API_CACHE));
    return;
  }

  if (
    request.destination === "script" ||
    request.destination === "style" ||
    request.destination === "image" ||
    request.destination === "font" ||
    request.destination === "manifest"
  ) {
    event.respondWith(staleWhileRevalidate(request, STATIC_CACHE));
    return;
  }

  if (
    url.pathname === "/" ||
    url.pathname.endsWith(".html") ||
    url.pathname.endsWith(".json") ||
    url.pathname === "/site.webmanifest"
  ) {
    event.respondWith(cacheFirst(request, SHELL_CACHE));
  }
});
