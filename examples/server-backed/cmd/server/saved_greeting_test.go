package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSavedGreetingHandlerStateTransitions(t *testing.T) {
	store := newSavedGreetingStore("GoFrame")
	handler := savedGreetingHandler(store, time.Millisecond)

	assertSavedGreetingResponse(t, handler, http.MethodGet, "", http.StatusOK, "GoFrame")
	assertSavedGreetingResponse(t, handler, http.MethodPost, "  Ada  ", http.StatusOK, "Ada")
	assertSavedGreetingResponse(t, handler, http.MethodGet, "", http.StatusOK, "Ada")

	assertSavedGreetingResponse(t, handler, http.MethodPost, "   ", http.StatusBadRequest, "name is required")
	assertSavedGreetingResponse(t, handler, http.MethodGet, "", http.StatusOK, "Ada")

	assertSavedGreetingResponse(t, handler, http.MethodPost, "fail", http.StatusInternalServerError, "controlled saved greeting failure")
	assertSavedGreetingResponse(t, handler, http.MethodGet, "", http.StatusOK, "Ada")
}

func TestSavedGreetingHandlerRejectsUnsupportedMethod(t *testing.T) {
	store := newSavedGreetingStore("GoFrame")
	handler := savedGreetingHandler(store, time.Millisecond)
	request := httptest.NewRequest(http.MethodPut, "/api/saved-greeting", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if got := response.Header().Get("Allow"); got != "GET, POST" {
		t.Fatalf("Allow = %q, want %q", got, "GET, POST")
	}
	if strings.TrimSpace(response.Body.String()) == "" {
		t.Fatal("unsupported-method response body is empty")
	}
}

func TestSavedGreetingHandlerCanceledSlowRequestDoesNotCommit(t *testing.T) {
	store := newSavedGreetingStore("GoFrame")
	delay := 500 * time.Millisecond
	handler := savedGreetingHandler(store, delay)
	started := make(chan struct{})
	body := &notifyingReader{
		Reader:  strings.NewReader(url.Values{"name": {"slow"}}.Encode()),
		started: started,
	}
	request := httptest.NewRequest(http.MethodPost, "/api/saved-greeting", body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})

	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("slow request body was not read")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("canceled slow request did not stop before the commit delay")
	}
	if got := store.load(); got != "GoFrame" {
		t.Fatalf("committed name = %q after cancellation, want GoFrame", got)
	}
}

func TestSavedGreetingHandlerConcurrentReadsAndWrites(t *testing.T) {
	store := newSavedGreetingStore("GoFrame")
	handler := savedGreetingHandler(store, time.Millisecond)
	names := []string{"Ada", "Grace", "Lin", "GoFrame"}
	valid := map[string]bool{"GoFrame": true, "Ada": true, "Grace": true, "Lin": true}
	start := make(chan struct{})
	errors := make(chan error, 64)
	var group sync.WaitGroup

	for index := 0; index < 64; index++ {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			if index%2 == 0 {
				name := names[index%len(names)]
				response := savedGreetingRequest(handler, http.MethodPost, name)
				if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != name {
					errors <- fmt.Errorf("POST %q returned status %d body %q", name, response.Code, response.Body.String())
				}
				return
			}
			response := savedGreetingRequest(handler, http.MethodGet, "")
			name := strings.TrimSpace(response.Body.String())
			if response.Code != http.StatusOK || !valid[name] {
				errors <- fmt.Errorf("GET returned status %d body %q", response.Code, response.Body.String())
			}
		}()
	}

	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

type notifyingReader struct {
	io.Reader
	started chan struct{}
	once    sync.Once
}

func (reader *notifyingReader) Read(buffer []byte) (int, error) {
	reader.once.Do(func() {
		close(reader.started)
	})
	return reader.Reader.Read(buffer)
}

func assertSavedGreetingResponse(
	t *testing.T,
	handler http.Handler,
	method string,
	name string,
	wantStatus int,
	wantBody string,
) {
	t.Helper()
	response := savedGreetingRequest(handler, method, name)
	if response.Code != wantStatus {
		t.Fatalf("%s %q status = %d, want %d; body=%q", method, name, response.Code, wantStatus, response.Body.String())
	}
	if got := strings.TrimSpace(response.Body.String()); got != wantBody {
		t.Fatalf("%s %q body = %q, want %q", method, name, got, wantBody)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("%s %q Cache-Control = %q, want no-store", method, name, got)
	}
}

func savedGreetingRequest(handler http.Handler, method string, name string) *httptest.ResponseRecorder {
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(url.Values{"name": {name}}.Encode())
	}
	request := httptest.NewRequest(method, "/api/saved-greeting", body)
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
