package main

import (
	"errors"
	"strings"
)

type serverBackedDocumentState struct {
	Title       string
	Description string
}

type serverBackedDocumentSnapshot struct {
	State       serverBackedDocumentState
	ActiveOwner string
	OwnerCount  int
}

type serverBackedDocumentOwner struct {
	key   string
	state serverBackedDocumentState
}

type serverBackedDocumentCoordinator struct {
	baseline serverBackedDocumentState
	owners   []serverBackedDocumentOwner
	index    map[string]int
}

func newServerBackedDocumentCoordinator(
	baseline serverBackedDocumentState,
) *serverBackedDocumentCoordinator {
	return &serverBackedDocumentCoordinator{
		baseline: baseline,
		index:    make(map[string]int),
	}
}

func (coordinator *serverBackedDocumentCoordinator) Snapshot() serverBackedDocumentSnapshot {
	if coordinator == nil {
		return serverBackedDocumentSnapshot{}
	}
	count := len(coordinator.owners)
	if count == 0 {
		return serverBackedDocumentSnapshot{State: coordinator.baseline}
	}
	active := coordinator.owners[count-1]
	return serverBackedDocumentSnapshot{
		State:       active.state,
		ActiveOwner: active.key,
		OwnerCount:  count,
	}
}

func (coordinator *serverBackedDocumentCoordinator) Set(
	owner string,
	state serverBackedDocumentState,
) (serverBackedDocumentSnapshot, bool, error) {
	if err := validateServerBackedDocumentOwner(owner); err != nil {
		return serverBackedDocumentSnapshot{}, false, err
	}
	if coordinator == nil {
		return serverBackedDocumentSnapshot{}, false, errors.New(
			"server-backed document coordinator is nil",
		)
	}
	if index, ok := coordinator.index[owner]; ok {
		if coordinator.owners[index].state == state {
			return coordinator.Snapshot(), false, nil
		}
		coordinator.owners[index].state = state
		return coordinator.Snapshot(), true, nil
	}
	coordinator.index[owner] = len(coordinator.owners)
	coordinator.owners = append(coordinator.owners, serverBackedDocumentOwner{
		key:   owner,
		state: state,
	})
	return coordinator.Snapshot(), true, nil
}

func (coordinator *serverBackedDocumentCoordinator) Remove(
	owner string,
) (serverBackedDocumentSnapshot, bool, error) {
	if err := validateServerBackedDocumentOwner(owner); err != nil {
		return serverBackedDocumentSnapshot{}, false, err
	}
	if coordinator == nil {
		return serverBackedDocumentSnapshot{}, false, errors.New(
			"server-backed document coordinator is nil",
		)
	}
	index, ok := coordinator.index[owner]
	if !ok {
		return coordinator.Snapshot(), false, nil
	}
	delete(coordinator.index, owner)
	copy(coordinator.owners[index:], coordinator.owners[index+1:])
	coordinator.owners = coordinator.owners[:len(coordinator.owners)-1]
	for next := index; next < len(coordinator.owners); next++ {
		coordinator.index[coordinator.owners[next].key] = next
	}
	return coordinator.Snapshot(), true, nil
}

func (coordinator *serverBackedDocumentCoordinator) OwnerKeys() []string {
	if coordinator == nil {
		return nil
	}
	keys := make([]string, len(coordinator.owners))
	for index, owner := range coordinator.owners {
		keys[index] = owner.key
	}
	return keys
}

func validateServerBackedDocumentOwner(owner string) error {
	if owner == "" || strings.TrimSpace(owner) != owner {
		return errors.New("server-backed document owner key must be non-empty and trimmed")
	}
	return nil
}

func homeDocumentMetadata() serverBackedDocumentState {
	return serverBackedDocumentState{
		Title:       "Server-backed Home · GoFrame",
		Description: "Choose a route-driven server-backed flow.",
	}
}

func greetingDocumentMetadata(name string) serverBackedDocumentState {
	return serverBackedDocumentState{
		Title:       "Greeting " + name + " · GoFrame",
		Description: "Backend greeting route for " + name + ".",
	}
}

func transitionDocumentMetadata(
	requestedName string,
	committed committedGreeting,
) serverBackedDocumentState {
	if committed.Ready {
		return serverBackedDocumentState{
			Title:       "Retained greeting: " + committed.Name + " · GoFrame",
			Description: "Committed retained greeting for " + committed.Name + ".",
		}
	}
	return serverBackedDocumentState{
		Title:       "Preparing retained greeting for " + requestedName + " · GoFrame",
		Description: "Preparing the first retained greeting for " + requestedName + ".",
	}
}

func savedGreetingDocumentMetadata(
	status string,
	value string,
) serverBackedDocumentState {
	switch status {
	case "ready":
		return serverBackedDocumentState{
			Title:       "Saved greeting: " + value + " · GoFrame",
			Description: "Committed saved greeting: " + value + ".",
		}
	case "failed":
		return serverBackedDocumentState{
			Title:       "Saved greeting unavailable · GoFrame",
			Description: "The committed saved greeting could not be loaded.",
		}
	default:
		return serverBackedDocumentState{
			Title:       "Saved greeting · GoFrame",
			Description: "Loading the committed saved greeting.",
		}
	}
}

func savedGreetingEditorDocumentMetadata(
	draft string,
	status string,
) serverBackedDocumentState {
	name := strings.TrimSpace(draft)
	if name == "" {
		name = "empty"
	}
	switch status {
	case "pending":
		return serverBackedDocumentState{
			Title:       "Saving greeting: " + name + " · GoFrame",
			Description: "Saving the greeting " + name + ".",
		}
	case "validation failed", "server failed":
		return serverBackedDocumentState{
			Title:       "Saved greeting needs attention · GoFrame",
			Description: "The draft " + name + " has not been committed.",
		}
	case "success":
		return serverBackedDocumentState{
			Title:       "Saved greeting confirmed: " + name + " · GoFrame",
			Description: "The server confirmed " + name + "; finish editing to reveal committed metadata.",
		}
	default:
		return serverBackedDocumentState{
			Title:       "Editing saved greeting: " + name + " · GoFrame",
			Description: "Unsaved saved-greeting draft: " + name + ".",
		}
	}
}

func notFoundDocumentMetadata() serverBackedDocumentState {
	return serverBackedDocumentState{
		Title:       "Not found · GoFrame",
		Description: "No server-backed route matched.",
	}
}
