package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type savedGreetingStore struct {
	mu   sync.RWMutex
	name string
}

func newSavedGreetingStore(initial string) *savedGreetingStore {
	return &savedGreetingStore{name: initial}
}

func (store *savedGreetingStore) load() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.name
}

func (store *savedGreetingStore) commit(name string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.name = name
}

func savedGreetingHandler(store *savedGreetingStore, slowDelay time.Duration) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")

		switch request.Method {
		case http.MethodGet:
			fmt.Fprint(response, store.load())
		case http.MethodPost:
			saveGreeting(response, request, store, slowDelay)
		default:
			response.Header().Set("Allow", "GET, POST")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func saveGreeting(
	response http.ResponseWriter,
	request *http.Request,
	store *savedGreetingStore,
	slowDelay time.Duration,
) {
	if err := request.ParseForm(); err != nil {
		http.Error(response, "invalid form body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(request.PostForm.Get("name"))
	if name == "" {
		http.Error(response, "name is required", http.StatusBadRequest)
		return
	}
	if name == "fail" {
		http.Error(response, "controlled saved greeting failure", http.StatusInternalServerError)
		return
	}
	if name == "slow" && !waitForSavedGreeting(request, slowDelay) {
		return
	}
	if request.Context().Err() != nil {
		return
	}

	store.commit(name)
	fmt.Fprint(response, name)
}

func waitForSavedGreeting(request *http.Request, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-request.Context().Done():
		return false
	}
}
