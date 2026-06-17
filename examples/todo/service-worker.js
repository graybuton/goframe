const CACHE_NAME = "goframe-todo-v1";
const ASSETS = ["./", "./index.html", "./assets/bundle.wasm", "./assets/wasm_exec.js"];

self.addEventListener("install", (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME).then((cache) =>
            Promise.allSettled(ASSETS.map((asset) => cache.add(asset))),
        ),
    );
    self.skipWaiting();
});

self.addEventListener("activate", (event) => {
    event.waitUntil(
        caches.keys().then((names) =>
            Promise.all(names.filter((name) => name !== CACHE_NAME).map((name) => caches.delete(name))),
        ),
    );
    self.clients.claim();
});

self.addEventListener("fetch", (event) => {
    if (event.request.method !== "GET") {
        return;
    }
    event.respondWith(caches.match(event.request).then((cached) => cached || fetch(event.request)));
});
