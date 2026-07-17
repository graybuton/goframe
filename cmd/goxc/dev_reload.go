package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	devEventsPath       = "/_goframe/dev/events"
	devReloadScriptPath = "/_goframe/dev/reload.js"
	devReloadMarker     = "data-goframe-dev-reload"
)

const devReloadClient = `(function () {
    var script = document.currentScript;
    var instance = script && script.getAttribute("data-goframe-instance");
    var generation = script && script.getAttribute("data-goframe-generation");
    if (!instance || !generation || typeof window.EventSource !== "function") return;
    var source = new EventSource("/_goframe/dev/events?instance=" + encodeURIComponent(instance) + "&generation=" + encodeURIComponent(generation));
    var reloading = false;
    source.addEventListener("reload", function () {
        if (reloading) return;
        reloading = true;
        source.close();
        window.location.reload();
    });
    window.addEventListener("beforeunload", function () { source.close(); }, { once: true });
})();
`

type devReloadBroker struct {
	mu             sync.Mutex
	instance       string
	current        uint64
	nextSubscriber uint64
	subscribers    map[uint64]*devReloadSubscriber
	closed         bool
}

type devReloadSubscriber struct {
	id              uint64
	events          chan uint64
	generationFloor uint64
}

type devReloadSubscription struct {
	broker *devReloadBroker
	id     uint64
	events <-chan uint64
	once   sync.Once
}

func newDevReloadInstance() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create development reload instance: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func newDevReloadBroker(instance string) *devReloadBroker {
	return &devReloadBroker{
		instance:    instance,
		subscribers: map[uint64]*devReloadSubscriber{},
	}
}

func (broker *devReloadBroker) activate(generation uint64, notify bool) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed || generation <= broker.current {
		return
	}
	broker.current = generation
	if !notify {
		return
	}
	for _, subscriber := range broker.subscribers {
		broker.queueLocked(subscriber, generation)
	}
}

func (broker *devReloadBroker) subscribe(instance string, generation uint64) (*devReloadSubscription, error) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed {
		return nil, errors.New("development reload broker is closed")
	}
	generationFloor := generation
	if instance != broker.instance {
		generationFloor = 0
	}
	broker.nextSubscriber++
	subscriber := &devReloadSubscriber{
		id:              broker.nextSubscriber,
		events:          make(chan uint64, 1),
		generationFloor: generationFloor,
	}
	broker.subscribers[subscriber.id] = subscriber
	if generationFloor < broker.current {
		broker.queueLocked(subscriber, broker.current)
	}
	return &devReloadSubscription{
		broker: broker,
		id:     subscriber.id,
		events: subscriber.events,
	}, nil
}

func (broker *devReloadBroker) queueLocked(subscriber *devReloadSubscriber, generation uint64) {
	if generation <= subscriber.generationFloor {
		return
	}
	select {
	case <-subscriber.events:
	default:
	}
	subscriber.events <- generation
	subscriber.generationFloor = generation
}

func (subscription *devReloadSubscription) Events() <-chan uint64 {
	return subscription.events
}

func (subscription *devReloadSubscription) Close() {
	subscription.once.Do(func() {
		subscription.broker.unsubscribe(subscription.id)
	})
}

func (broker *devReloadBroker) unsubscribe(id uint64) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	subscriber, ok := broker.subscribers[id]
	if !ok {
		return
	}
	delete(broker.subscribers, id)
	close(subscriber.events)
}

func (broker *devReloadBroker) subscriberCount() int {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	return len(broker.subscribers)
}

func (broker *devReloadBroker) close() {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed {
		return
	}
	broker.closed = true
	for id, subscriber := range broker.subscribers {
		delete(broker.subscribers, id)
		close(subscriber.events)
	}
}

func devReloadHandler(generations *devGenerationManager, broker *devReloadBroker) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case devEventsPath:
			serveDevEvents(response, request, broker)
			return
		case devReloadScriptPath:
			serveDevReloadClient(response, request)
			return
		}

		lease, err := generations.acquire()
		if err != nil {
			http.Error(response, "development package is not ready", http.StatusServiceUnavailable)
			return
		}
		defer lease.Release()
		if request.Method == http.MethodGet || request.Method == http.MethodHead {
			if path, err := sanitizeServePath(request.URL.Path, request.URL.RawPath); err == nil && (path == "/" || path == "/index.html") {
				serveDevIndex(response, request, lease, broker.instance)
				return
			}
		}
		devStaticHandler(lease.Directory()).ServeHTTP(response, request)
	})
}

func serveDevEvents(response http.ResponseWriter, request *http.Request, broker *devReloadBroker) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := request.URL.Query()
	instance := query.Get("instance")
	if instance == "" {
		http.Error(response, "invalid development instance", http.StatusBadRequest)
		return
	}
	generation, err := strconv.ParseUint(query.Get("generation"), 10, 64)
	if err != nil || generation == 0 {
		http.Error(response, "invalid development generation", http.StatusBadRequest)
		return
	}
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	subscription, err := broker.subscribe(instance, generation)
	if err != nil {
		http.Error(response, "development reload is shutting down", http.StatusServiceUnavailable)
		return
	}
	defer subscription.Close()

	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(response, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-request.Context().Done():
			return
		case generation, ok := <-subscription.Events():
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(response, "event: reload\ndata: %d\n\n", generation); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func serveDevReloadClient(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	response.Header().Set("Content-Length", strconv.Itoa(len(devReloadClient)))
	response.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = response.Write([]byte(devReloadClient))
	}
}

func serveDevIndex(response http.ResponseWriter, request *http.Request, lease *devGenerationLease, instance string) {
	indexPath := filepath.Join(lease.Directory(), indexHTMLAssetName)
	if err := validatePathBelowRoot(lease.Directory(), indexPath, "development index", false); err != nil {
		http.NotFound(response, request)
		return
	}
	if _, err := regularFileNoFollow(indexPath, "development index"); err != nil {
		http.NotFound(response, request)
		return
	}
	content, err := os.ReadFile(indexPath)
	if err != nil {
		http.Error(response, "read development index", http.StatusInternalServerError)
		return
	}
	injected := injectDevReloadClient(string(content), instance, lease.ID())
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Content-Length", strconv.Itoa(len(injected)))
	response.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = response.Write([]byte(injected))
	}
}

func injectDevReloadClient(content, instance string, generation uint64) string {
	if strings.Contains(content, devReloadMarker) {
		return content
	}
	tag := fmt.Sprintf(`<script %s src="%s" data-goframe-instance="%s" data-goframe-generation="%d"></script>`, devReloadMarker, devReloadScriptPath, instance, generation)
	if index := lastASCIIFoldIndex(content, "</body>"); index >= 0 {
		return content[:index] + tag + "\n" + content[index:]
	}
	if content == "" || strings.HasSuffix(content, "\n") {
		return content + tag + "\n"
	}
	return content + "\n" + tag + "\n"
}

func lastASCIIFoldIndex(content, target string) int {
	if target == "" {
		return len(content)
	}
	for index := len(content) - len(target); index >= 0; index-- {
		matched := true
		for offset := range len(target) {
			left := content[index+offset]
			right := target[offset]
			if left >= 'A' && left <= 'Z' {
				left += 'a' - 'A'
			}
			if right >= 'A' && right <= 'Z' {
				right += 'a' - 'A'
			}
			if left != right {
				matched = false
				break
			}
		}
		if matched {
			return index
		}
	}
	return -1
}
