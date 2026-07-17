package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDevReloadSameGenerationConnectionWaits(t *testing.T) {
	broker := newDevReloadBroker()
	broker.activate(1, false)
	subscription := mustDevReloadSubscription(t, broker, 1)
	defer subscription.Close()
	assertNoQueuedDevReload(t, subscription)
}

func TestDevReloadStaleGenerationReceivesCatchUp(t *testing.T) {
	broker := newDevReloadBroker()
	broker.activate(3, false)
	subscription := mustDevReloadSubscription(t, broker, 1)
	defer subscription.Close()
	assertDevReloadGeneration(t, subscription, 3)
	assertNoQueuedDevReload(t, subscription)
}

func TestDevReloadPublishesToTwoSubscribersOnce(t *testing.T) {
	broker := newDevReloadBroker()
	broker.activate(1, false)
	first := mustDevReloadSubscription(t, broker, 1)
	second := mustDevReloadSubscription(t, broker, 1)
	defer first.Close()
	defer second.Close()

	broker.activate(2, true)
	assertDevReloadGeneration(t, first, 2)
	assertDevReloadGeneration(t, second, 2)
	broker.activate(2, true)
	assertNoQueuedDevReload(t, first)
	assertNoQueuedDevReload(t, second)
}

func TestDevReloadSlowSubscriberCollapsesToNewestGeneration(t *testing.T) {
	broker := newDevReloadBroker()
	broker.activate(1, false)
	subscription := mustDevReloadSubscription(t, broker, 1)
	defer subscription.Close()

	broker.activate(2, true)
	broker.activate(3, true)
	broker.activate(4, true)
	assertDevReloadGeneration(t, subscription, 4)
	assertNoQueuedDevReload(t, subscription)
}

func TestDevReloadDisconnectAndShutdownReleaseSubscribers(t *testing.T) {
	broker := newDevReloadBroker()
	first := mustDevReloadSubscription(t, broker, 0)
	second := mustDevReloadSubscription(t, broker, 0)
	if got := broker.subscriberCount(); got != 2 {
		t.Fatalf("subscriber count = %d, want 2", got)
	}
	first.Close()
	if got := broker.subscriberCount(); got != 1 {
		t.Fatalf("subscriber count after disconnect = %d, want 1", got)
	}
	broker.close()
	if got := broker.subscriberCount(); got != 0 {
		t.Fatalf("subscriber count after shutdown = %d, want 0", got)
	}
	if _, ok := <-second.Events(); ok {
		t.Fatal("subscriber channel remained open after shutdown")
	}
	second.Close()
}

func TestDevReloadConcurrentPublishSubscribeDisconnect(t *testing.T) {
	broker := newDevReloadBroker()
	broker.activate(1, false)
	var workers sync.WaitGroup
	for worker := 0; worker < 12; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for iteration := 0; iteration < 100; iteration++ {
				subscription, err := broker.subscribe(uint64(worker % 4))
				if err != nil {
					return
				}
				select {
				case <-subscription.Events():
				default:
				}
				subscription.Close()
			}
		}(worker)
	}
	for generation := uint64(2); generation < 80; generation++ {
		broker.activate(generation, true)
	}
	workers.Wait()
	broker.close()
	if got := broker.subscriberCount(); got != 0 {
		t.Fatalf("subscriber count = %d, want 0", got)
	}
}

func TestDevReloadEventsEndpointStreamsReload(t *testing.T) {
	broker := newDevReloadBroker()
	broker.activate(1, false)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		serveDevEvents(response, request, broker)
	}))
	defer server.Close()

	response, err := http.Get(server.URL + "?generation=1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	reader := bufio.NewReader(response.Body)
	if line, err := reader.ReadString('\n'); err != nil || line != ": connected\n" {
		t.Fatalf("initial SSE line = %q, %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "\n" {
		t.Fatalf("initial SSE separator = %q, %v", line, err)
	}

	broker.activate(2, true)
	for _, want := range []string{"event: reload\n", "data: 2\n", "\n"} {
		if line, err := reader.ReadString('\n'); err != nil || line != want {
			t.Fatalf("SSE line = %q, %v, want %q", line, err, want)
		}
	}
	response.Body.Close()
	waitForDevReloadSubscriberCount(t, broker, 0)
}

func TestDevReloadEventsEndpointRejectsUnsupportedMethod(t *testing.T) {
	response := httptest.NewRecorder()
	serveDevEvents(response, httptest.NewRequest(http.MethodPost, devEventsPath+"?generation=1", nil), newDevReloadBroker())
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("response = %d Allow=%q, want 405 GET", response.Code, response.Header().Get("Allow"))
	}
}

func TestDevReloadHandlerInjectsOneGenerationClient(t *testing.T) {
	packageDir := t.TempDir()
	canonicalIndex := "<!doctype html><html><body><main>app</main></body></html>"
	writeDevGenerationPackage(t, packageDir, canonicalIndex)
	manager := newTestDevGenerationManager(t)
	generation, err := manager.activatePackage(packageDir)
	if err != nil {
		t.Fatal(err)
	}
	broker := newDevReloadBroker()
	broker.activate(generation, false)
	handler := devReloadHandler(manager, broker)

	for _, requestPath := range []string{"/", "/index.html"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", requestPath, response.Code)
		}
		body := response.Body.String()
		if strings.Count(body, devReloadMarker) != 1 {
			t.Fatalf("GET %s reload marker count = %d, body=%q", requestPath, strings.Count(body, devReloadMarker), body)
		}
		wantTag := fmt.Sprintf(`%s src="%s" data-goframe-generation="%d"`, devReloadMarker, devReloadScriptPath, generation)
		if !strings.Contains(body, wantTag) || strings.Index(body, wantTag) > strings.Index(strings.ToLower(body), "</body>") {
			t.Fatalf("GET %s did not inject generation client before body: %q", requestPath, body)
		}
		if got := response.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("GET %s Cache-Control = %q", requestPath, got)
		}
	}
	assertFileContent(t, filepath.Join(packageDir, indexHTMLAssetName), canonicalIndex)

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q, want 200 with no body", head.Code, head.Body.String())
	}
	if length, err := strconv.Atoi(head.Header().Get("Content-Length")); err != nil || length <= len(canonicalIndex) {
		t.Fatalf("HEAD Content-Length = %q, want injected length", head.Header().Get("Content-Length"))
	}
}

func TestDevReloadHandlerAppendsClientWithoutClosingBody(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "<main>app</main>")
	manager := newTestDevGenerationManager(t)
	generation, err := manager.activatePackage(packageDir)
	if err != nil {
		t.Fatal(err)
	}
	broker := newDevReloadBroker()
	broker.activate(generation, false)
	response := httptest.NewRecorder()
	devReloadHandler(manager, broker).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.HasSuffix(response.Body.String(), "</script>\n") || strings.Count(response.Body.String(), devReloadMarker) != 1 {
		t.Fatalf("injected body = %q", response.Body.String())
	}
}

func TestDevReloadClientIsDevelopmentOnlyResponse(t *testing.T) {
	response := httptest.NewRecorder()
	serveDevReloadClient(response, httptest.NewRequest(http.MethodGet, devReloadScriptPath, nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("reload client response = %d Cache-Control=%q", response.Code, response.Header().Get("Cache-Control"))
	}
	for _, want := range []string{"EventSource", "data-goframe-generation", "window.location.reload()"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("reload client missing %q: %q", want, response.Body.String())
		}
	}

	head := httptest.NewRecorder()
	serveDevReloadClient(head, httptest.NewRequest(http.MethodHead, devReloadScriptPath, nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD reload client = %d %q", head.Code, head.Body.String())
	}
}

func mustDevReloadSubscription(t *testing.T, broker *devReloadBroker, generation uint64) *devReloadSubscription {
	t.Helper()
	subscription, err := broker.subscribe(generation)
	if err != nil {
		t.Fatal(err)
	}
	return subscription
}

func assertDevReloadGeneration(t *testing.T, subscription *devReloadSubscription, want uint64) {
	t.Helper()
	select {
	case got, ok := <-subscription.Events():
		if !ok || got != want {
			t.Fatalf("reload event = %d, %v, want %d, true", got, ok, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for generation %d", want)
	}
}

func assertNoQueuedDevReload(t *testing.T, subscription *devReloadSubscription) {
	t.Helper()
	select {
	case generation, ok := <-subscription.Events():
		t.Fatalf("unexpected reload event = %d, %v", generation, ok)
	default:
	}
}

func waitForDevReloadSubscriberCount(t *testing.T, broker *devReloadBroker, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if broker.subscriberCount() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("subscriber count = %d, want %d", broker.subscriberCount(), want)
}
