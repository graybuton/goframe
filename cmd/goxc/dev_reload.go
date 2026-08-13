package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	devEventsPath          = "/_goframe/dev/events"
	devReloadScriptPath    = "/_goframe/dev/reload.js"
	devReloadMarker        = "data-goframe-dev-reload"
	devBuildErrorMarker    = "data-goframe-dev-build-error"
	devReloadEventName     = "reload"
	devBuildErrorEventName = "build-error"
)

type devReloadEventKind uint8

const (
	devReloadEventGeneration devReloadEventKind = iota + 1
	devReloadEventBuildError
)

const devReloadClient = `(function () {
    var script = document.currentScript;
    var instance = script && script.getAttribute("data-goframe-instance");
    var generation = script && script.getAttribute("data-goframe-generation");
    if (!instance || !generation || typeof window.EventSource !== "function") return;
    var source = new EventSource("/_goframe/dev/events?instance=" + encodeURIComponent(instance) + "&generation=" + encodeURIComponent(generation));
    var reloading = false;
    var buildErrorPanel = null;
    var buildErrorHeading = null;
    var buildErrorMessage = null;
    function clearBuildErrorReferences() {
        buildErrorPanel = null;
        buildErrorHeading = null;
        buildErrorMessage = null;
    }
    function ensureBuildErrorPanel() {
        if (buildErrorPanel && buildErrorPanel.isConnected) return;
        clearBuildErrorReferences();
        buildErrorPanel = document.createElement("section");
        buildErrorPanel.setAttribute("data-goframe-dev-build-error", "");
        buildErrorPanel.setAttribute("role", "alert");
        buildErrorPanel.setAttribute("aria-live", "assertive");
        buildErrorPanel.setAttribute("aria-atomic", "true");
        buildErrorPanel.style.cssText = "position:fixed;right:1rem;bottom:1rem;z-index:2147483647;box-sizing:border-box;width:min(42rem,calc(100vw - 2rem));max-height:min(50vh,32rem);overflow:auto;padding:1rem;border:2px solid #f87171;border-radius:.5rem;background:#1f1013;color:#fff;font:14px/1.45 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;box-shadow:0 1rem 3rem rgba(0,0,0,.45);";
        buildErrorHeading = document.createElement("strong");
        buildErrorHeading.setAttribute("data-goframe-dev-build-error-heading", "");
        buildErrorHeading.style.cssText = "display:block;margin:0 0 .75rem;color:#fca5a5;font:600 15px/1.3 system-ui,sans-serif;";
        buildErrorMessage = document.createElement("pre");
        buildErrorMessage.setAttribute("data-goframe-dev-build-error-message", "");
        buildErrorMessage.style.cssText = "margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit;";
        buildErrorPanel.appendChild(buildErrorHeading);
        buildErrorPanel.appendChild(buildErrorMessage);
        (document.body || document.documentElement).appendChild(buildErrorPanel);
    }
    function removeBuildError() {
        if (buildErrorPanel && buildErrorPanel.isConnected) buildErrorPanel.remove();
        clearBuildErrorReferences();
    }
    function showBuildError(event) {
        var failure;
        try {
            failure = JSON.parse(event.data);
        } catch (_) {
            return;
        }
        if (!failure || !Number.isInteger(failure.build) || failure.build <= 0 || typeof failure.message !== "string") return;
        ensureBuildErrorPanel();
        buildErrorPanel.setAttribute("data-goframe-dev-build", String(failure.build));
        buildErrorHeading.textContent = "Build " + failure.build + " failed";
        buildErrorMessage.textContent = failure.message;
    }
    source.addEventListener("build-error", showBuildError);
    source.addEventListener("reload", function () {
        if (reloading) return;
        reloading = true;
        removeBuildError();
        source.close();
        window.location.reload();
    });
    window.addEventListener("beforeunload", function () { source.close(); }, { once: true });
})();
`

type devBuildError struct {
	Build   int    `json:"build"`
	Message string `json:"message"`
}

type devReloadEvent struct {
	kind       devReloadEventKind
	generation uint64
	buildError devBuildError
}

type devReloadBroker struct {
	mu             sync.Mutex
	instance       string
	current        uint64
	latestBuild    int
	buildError     *devBuildError
	nextSubscriber uint64
	subscribers    map[uint64]*devReloadSubscriber
	closed         bool
}

type devReloadSubscriber struct {
	id              uint64
	events          chan devReloadEvent
	generationFloor uint64
	reloadRequired  bool
}

type devReloadSubscription struct {
	broker *devReloadBroker
	id     uint64
	events <-chan devReloadEvent
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
	broker.activateBuild(generation, 0, notify)
}

func (broker *devReloadBroker) activateBuild(generation uint64, build int, notify bool) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed || generation <= broker.current {
		return
	}
	if build > 0 && build <= broker.latestBuild {
		return
	}
	broker.current = generation
	if build > 0 {
		broker.latestBuild = build
	}
	broker.buildError = nil
	if !notify {
		return
	}
	for _, subscriber := range broker.subscribers {
		broker.queueReloadLocked(subscriber, generation)
	}
}

func (broker *devReloadBroker) publishBuildError(build int, err error) {
	if err == nil {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed || build <= broker.latestBuild {
		return
	}
	broker.latestBuild = build
	failure := devBuildError{Build: build, Message: err.Error()}
	broker.buildError = &failure
	for _, subscriber := range broker.subscribers {
		broker.queueBuildErrorLocked(subscriber, failure)
	}
}

func (broker *devReloadBroker) currentBuildError() (devBuildError, bool) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.buildError == nil {
		return devBuildError{}, false
	}
	return *broker.buildError, true
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
		events:          make(chan devReloadEvent, 1),
		generationFloor: generationFloor,
	}
	broker.subscribers[subscriber.id] = subscriber
	if generationFloor < broker.current {
		broker.queueReloadLocked(subscriber, broker.current)
	} else if generationFloor == broker.current && broker.buildError != nil {
		broker.queueBuildErrorLocked(subscriber, *broker.buildError)
	}
	return &devReloadSubscription{
		broker: broker,
		id:     subscriber.id,
		events: subscriber.events,
	}, nil
}

func (broker *devReloadBroker) queueReloadLocked(subscriber *devReloadSubscriber, generation uint64) {
	if generation <= subscriber.generationFloor {
		return
	}
	broker.replaceQueuedEventLocked(subscriber, newDevReloadEvent(generation))
	subscriber.generationFloor = generation
	subscriber.reloadRequired = true
}

func (broker *devReloadBroker) queueBuildErrorLocked(subscriber *devReloadSubscriber, failure devBuildError) {
	if subscriber.reloadRequired || subscriber.generationFloor != broker.current {
		return
	}
	broker.replaceQueuedEventLocked(subscriber, newDevBuildErrorEvent(failure))
}

func (broker *devReloadBroker) replaceQueuedEventLocked(subscriber *devReloadSubscriber, event devReloadEvent) {
	select {
	case <-subscriber.events:
	default:
	}
	subscriber.events <- event
}

func newDevReloadEvent(generation uint64) devReloadEvent {
	return devReloadEvent{kind: devReloadEventGeneration, generation: generation}
}

func newDevBuildErrorEvent(failure devBuildError) devReloadEvent {
	return devReloadEvent{kind: devReloadEventBuildError, buildError: failure}
}

func (subscription *devReloadSubscription) Events() <-chan devReloadEvent {
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
	broker.buildError = nil
	for id, subscriber := range broker.subscribers {
		delete(broker.subscribers, id)
		select {
		case <-subscriber.events:
		default:
		}
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
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}
			if err := writeDevReloadEvent(response, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeDevReloadEvent(output io.Writer, event devReloadEvent) error {
	switch event.kind {
	case devReloadEventGeneration:
		_, err := fmt.Fprintf(output, "event: %s\ndata: %d\n\n", devReloadEventName, event.generation)
		return err
	case devReloadEventBuildError:
		payload, err := json.Marshal(event.buildError)
		if err != nil {
			return fmt.Errorf("encode development build error: %w", err)
		}
		_, err = fmt.Fprintf(output, "event: %s\ndata: %s\n\n", devBuildErrorEventName, payload)
		return err
	default:
		return errors.New("unknown development reload event")
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
