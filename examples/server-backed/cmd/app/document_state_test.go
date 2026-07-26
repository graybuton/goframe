package main

import (
	"reflect"
	"testing"
)

func TestDocumentStateCoordinatorRetainsLatestRemainingOwner(t *testing.T) {
	baseline := serverBackedDocumentState{
		Title:       "authored",
		Description: "baseline",
	}
	routeInitial := serverBackedDocumentState{
		Title:       "Saved greeting: GoFrame · GoFrame",
		Description: "Committed saved greeting: GoFrame.",
	}
	editorInitial := serverBackedDocumentState{
		Title:       "Editing saved greeting: slow · GoFrame",
		Description: "Unsaved saved-greeting draft: slow.",
	}
	routeUpdated := serverBackedDocumentState{
		Title:       "Saved greeting: Grace · GoFrame",
		Description: "Committed saved greeting: Grace.",
	}
	coordinator := newServerBackedDocumentCoordinator(baseline)

	assertDocumentSnapshot(t, coordinator.Snapshot(), serverBackedDocumentSnapshot{
		State: baseline,
	})
	mustSetDocumentOwner(t, coordinator, "route", routeInitial)
	mustSetDocumentOwner(t, coordinator, "saved-editor", editorInitial)

	snapshot, changed, err := coordinator.Set("route", routeUpdated)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("route update was ignored")
	}
	assertDocumentSnapshot(t, snapshot, serverBackedDocumentSnapshot{
		State:       editorInitial,
		ActiveOwner: "saved-editor",
		OwnerCount:  2,
	})
	if got := coordinator.OwnerKeys(); !reflect.DeepEqual(got, []string{"route", "saved-editor"}) {
		t.Fatalf("owner order after route update = %v", got)
	}

	editorUpdated := savedGreetingEditorDocumentMetadata(
		"Grace",
		"success",
		savedGreetingMutationTarget{
			Submitted: "Grace",
			Confirmed: "Grace",
		},
	)
	mustSetDocumentOwner(t, coordinator, "saved-editor", editorUpdated)
	snapshot = mustRemoveDocumentOwner(t, coordinator, "saved-editor")
	assertDocumentSnapshot(t, snapshot, serverBackedDocumentSnapshot{
		State:       routeUpdated,
		ActiveOwner: "route",
		OwnerCount:  1,
	})

	snapshot = mustRemoveDocumentOwner(t, coordinator, "route")
	assertDocumentSnapshot(t, snapshot, serverBackedDocumentSnapshot{
		State: baseline,
	})
}

func TestDocumentStateCoordinatorNonTopRemovalAndNoOpUpdates(t *testing.T) {
	coordinator := newServerBackedDocumentCoordinator(serverBackedDocumentState{
		Title:       "authored",
		Description: "baseline",
	})
	route := serverBackedDocumentState{Title: "route", Description: "route"}
	editor := serverBackedDocumentState{Title: "editor", Description: "editor"}
	mustSetDocumentOwner(t, coordinator, "route", route)
	mustSetDocumentOwner(t, coordinator, "saved-editor", editor)

	snapshot, changed, err := coordinator.Set("route", route)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("identical route update reported a change")
	}
	assertDocumentSnapshot(t, snapshot, serverBackedDocumentSnapshot{
		State:       editor,
		ActiveOwner: "saved-editor",
		OwnerCount:  2,
	})

	snapshot = mustRemoveDocumentOwner(t, coordinator, "route")
	assertDocumentSnapshot(t, snapshot, serverBackedDocumentSnapshot{
		State:       editor,
		ActiveOwner: "saved-editor",
		OwnerCount:  1,
	})
}

func TestDocumentStateCoordinatorRejectsMalformedOwnerKeys(t *testing.T) {
	coordinator := newServerBackedDocumentCoordinator(serverBackedDocumentState{})
	for _, owner := range []string{"", " ", "\troute", "route "} {
		if _, _, err := coordinator.Set(owner, serverBackedDocumentState{}); err == nil {
			t.Fatalf("Set(%q) succeeded", owner)
		}
		if _, _, err := coordinator.Remove(owner); err == nil {
			t.Fatalf("Remove(%q) succeeded", owner)
		}
	}
}

func TestDocumentStateOwnerIdentityDoesNotChangePriority(t *testing.T) {
	coordinator := newServerBackedDocumentCoordinator(serverBackedDocumentState{})
	mustSetDocumentOwner(t, coordinator, "route", serverBackedDocumentState{
		Title: "first",
	})
	mustSetDocumentOwner(t, coordinator, "saved-editor", serverBackedDocumentState{
		Title: "editor",
	})
	mustSetDocumentOwner(t, coordinator, "route", serverBackedDocumentState{
		Title: "latest",
	})

	if got := coordinator.OwnerKeys(); !reflect.DeepEqual(got, []string{"route", "saved-editor"}) {
		t.Fatalf("owner update changed fixed owner priority: %v", got)
	}
}

func TestDocumentStateCoordinatorDoesNotMutateCallerValues(t *testing.T) {
	baseline := serverBackedDocumentState{Title: "authored", Description: "baseline"}
	desired := serverBackedDocumentState{Title: "route", Description: "selected"}
	coordinator := newServerBackedDocumentCoordinator(baseline)
	mustSetDocumentOwner(t, coordinator, "route", desired)

	if baseline != (serverBackedDocumentState{Title: "authored", Description: "baseline"}) {
		t.Fatalf("baseline changed: %#v", baseline)
	}
	if desired != (serverBackedDocumentState{Title: "route", Description: "selected"}) {
		t.Fatalf("desired state changed: %#v", desired)
	}
}

func TestDocumentMetadataMapping(t *testing.T) {
	tests := []struct {
		name string
		got  serverBackedDocumentState
		want serverBackedDocumentState
	}{
		{
			name: "home",
			got:  homeDocumentMetadata(),
			want: serverBackedDocumentState{
				Title:       "Server-backed Home · GoFrame",
				Description: "Choose a route-driven server-backed flow.",
			},
		},
		{
			name: "greeting",
			got:  greetingDocumentMetadata("Ada"),
			want: serverBackedDocumentState{
				Title:       "Greeting Ada · GoFrame",
				Description: "Backend greeting route for Ada.",
			},
		},
		{
			name: "transition initial",
			got:  transitionDocumentMetadata("Ada", committedGreeting{}),
			want: serverBackedDocumentState{
				Title:       "Preparing retained greeting for Ada · GoFrame",
				Description: "Preparing the first retained greeting for Ada.",
			},
		},
		{
			name: "transition committed ignores requested slow",
			got: transitionDocumentMetadata("slow", committedGreeting{
				Name:    "Ada",
				Target:  "/transition-greeting?name=Ada",
				Message: "Hello, Ada, from Go backend!",
				Ready:   true,
			}),
			want: serverBackedDocumentState{
				Title:       "Retained greeting: Ada · GoFrame",
				Description: "Committed retained greeting for Ada.",
			},
		},
		{
			name: "transition committed Lin",
			got: transitionDocumentMetadata("Lin", committedGreeting{
				Name:    "Lin",
				Target:  "/transition-greeting?name=Lin",
				Message: "Hello, Lin, from Go backend!",
				Ready:   true,
			}),
			want: serverBackedDocumentState{
				Title:       "Retained greeting: Lin · GoFrame",
				Description: "Committed retained greeting for Lin.",
			},
		},
		{
			name: "saved loading",
			got:  savedGreetingDocumentMetadata("loading", ""),
			want: serverBackedDocumentState{
				Title:       "Saved greeting · GoFrame",
				Description: "Loading the committed saved greeting.",
			},
		},
		{
			name: "saved ready",
			got:  savedGreetingDocumentMetadata("ready", "GoFrame"),
			want: serverBackedDocumentState{
				Title:       "Saved greeting: GoFrame · GoFrame",
				Description: "Committed saved greeting: GoFrame.",
			},
		},
		{
			name: "saved failure",
			got:  savedGreetingDocumentMetadata("failed", ""),
			want: serverBackedDocumentState{
				Title:       "Saved greeting unavailable · GoFrame",
				Description: "The committed saved greeting could not be loaded.",
			},
		},
		{
			name: "saved editor",
			got: savedGreetingEditorDocumentMetadata(
				"slow",
				"idle",
				savedGreetingMutationTarget{},
			),
			want: serverBackedDocumentState{
				Title:       "Editing saved greeting: slow · GoFrame",
				Description: "Unsaved saved-greeting draft: slow.",
			},
		},
		{
			name: "saved editor pending",
			got: savedGreetingEditorDocumentMetadata(
				"slow",
				"pending",
				savedGreetingMutationTarget{Submitted: "slow"},
			),
			want: serverBackedDocumentState{
				Title:       "Saving greeting: slow · GoFrame",
				Description: "Saving the greeting slow.",
			},
		},
		{
			name: "saved editor validation failure",
			got: savedGreetingEditorDocumentMetadata(
				"   ",
				"validation failed",
				savedGreetingMutationTarget{},
			),
			want: serverBackedDocumentState{
				Title:       "Saved greeting needs attention · GoFrame",
				Description: "The draft empty has not been committed.",
			},
		},
		{
			name: "saved editor server failure",
			got: savedGreetingEditorDocumentMetadata(
				"fail",
				"server failed",
				savedGreetingMutationTarget{Submitted: "fail"},
			),
			want: serverBackedDocumentState{
				Title:       "Saved greeting needs attention · GoFrame",
				Description: "The draft fail has not been committed.",
			},
		},
		{
			name: "saved editor confirmed",
			got: savedGreetingEditorDocumentMetadata(
				"Grace",
				"success",
				savedGreetingMutationTarget{
					Submitted: "Grace",
					Confirmed: "Grace",
				},
			),
			want: serverBackedDocumentState{
				Title:       "Saved greeting confirmed: Grace · GoFrame",
				Description: "The server confirmed Grace; finish editing to reveal committed metadata.",
			},
		},
		{
			name: "not found",
			got:  notFoundDocumentMetadata(),
			want: serverBackedDocumentState{
				Title:       "Not found · GoFrame",
				Description: "No server-backed route matched.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("metadata = %#v, want %#v", test.got, test.want)
			}
		})
	}
}

func TestSavedMutationDocumentMetadataAttribution(t *testing.T) {
	tests := []struct {
		name   string
		draft  string
		status string
		target savedGreetingMutationTarget
		want   serverBackedDocumentState
	}{
		{
			name:   "pending keeps submitted target after draft edit",
			draft:  "Mia",
			status: "pending",
			target: savedGreetingMutationTarget{Submitted: "slow"},
			want: serverBackedDocumentState{
				Title:       "Saving greeting: slow · GoFrame",
				Description: "Saving the greeting slow.",
			},
		},
		{
			name:   "success uses confirmed response after draft edit",
			draft:  "Mia",
			status: "success",
			target: savedGreetingMutationTarget{
				Submitted: "slow",
				Confirmed: "slow",
			},
			want: serverBackedDocumentState{
				Title:       "Saved greeting confirmed: slow · GoFrame",
				Description: "The server confirmed slow; finish editing to reveal committed metadata.",
			},
		},
		{
			name:   "success prefers normalized server response to submitted target",
			draft:  "Mia",
			status: "success",
			target: savedGreetingMutationTarget{
				Submitted: "submitted",
				Confirmed: "confirmed",
			},
			want: serverBackedDocumentState{
				Title:       "Saved greeting confirmed: confirmed · GoFrame",
				Description: "The server confirmed confirmed; finish editing to reveal committed metadata.",
			},
		},
		{
			name:   "server failure keeps submitted target after draft edit",
			draft:  "Mia",
			status: "server failed",
			target: savedGreetingMutationTarget{Submitted: "fail"},
			want: serverBackedDocumentState{
				Title:       "Saved greeting needs attention · GoFrame",
				Description: "The draft fail has not been committed.",
			},
		},
		{
			name:   "validation failure follows current invalid draft",
			draft:  "   ",
			status: "validation failed",
			target: savedGreetingMutationTarget{},
			want: serverBackedDocumentState{
				Title:       "Saved greeting needs attention · GoFrame",
				Description: "The draft empty has not been committed.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := savedGreetingEditorDocumentMetadata(
				test.draft,
				test.status,
				test.target,
			)
			if got != test.want {
				t.Fatalf("metadata = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSavedMutationConfirmationNormalizesResponse(t *testing.T) {
	target, ok := confirmedSavedGreetingMutationTarget(
		savedGreetingMutationTarget{Submitted: "submitted"},
		"  confirmed  ",
	)
	if !ok {
		t.Fatal("non-empty server response was rejected")
	}
	if target != (savedGreetingMutationTarget{
		Submitted: "submitted",
		Confirmed: "confirmed",
	}) {
		t.Fatalf("confirmed target = %#v", target)
	}

	target, ok = confirmedSavedGreetingMutationTarget(target, " \t ")
	if ok {
		t.Fatal("empty server response was accepted")
	}
	if target.Confirmed != "" {
		t.Fatalf("empty response retained confirmation %q", target.Confirmed)
	}
}

func TestSavedMutationInputState(t *testing.T) {
	activeTarget := savedGreetingMutationTarget{Submitted: "slow"}
	status, mutationError, target, changed := savedGreetingMutationAfterInput(
		true,
		"pending",
		"",
		activeTarget,
	)
	if changed ||
		status != "pending" ||
		mutationError != "" ||
		target != activeTarget {
		t.Fatalf(
			"active mutation changed after input: status=%q error=%q target=%#v changed=%v",
			status,
			mutationError,
			target,
			changed,
		)
	}

	completedTarget := savedGreetingMutationTarget{
		Submitted: "slow",
		Confirmed: "slow",
	}
	status, mutationError, target, changed = savedGreetingMutationAfterInput(
		false,
		"success",
		"obsolete",
		completedTarget,
	)
	if !changed ||
		status != "idle" ||
		mutationError != "" ||
		target != (savedGreetingMutationTarget{}) {
		t.Fatalf(
			"completed mutation was not cleared after input: status=%q error=%q target=%#v changed=%v",
			status,
			mutationError,
			target,
			changed,
		)
	}

	status, mutationError, target, changed = savedGreetingMutationAfterInput(
		false,
		"idle",
		"",
		savedGreetingMutationTarget{},
	)
	if changed ||
		status != "idle" ||
		mutationError != "" ||
		target != (savedGreetingMutationTarget{}) {
		t.Fatalf(
			"idle mutation changed after input: status=%q error=%q target=%#v changed=%v",
			status,
			mutationError,
			target,
			changed,
		)
	}
}

func TestDocumentStateIntegratedSavedEditorReveal(t *testing.T) {
	coordinator := newServerBackedDocumentCoordinator(serverBackedDocumentState{
		Title:       "authored",
		Description: "baseline",
	})
	mustSetDocumentOwner(
		t,
		coordinator,
		"route",
		savedGreetingDocumentMetadata("ready", "GoFrame"),
	)
	mustSetDocumentOwner(
		t,
		coordinator,
		"saved-editor",
		savedGreetingEditorDocumentMetadata(
			"Grace",
			"pending",
			savedGreetingMutationTarget{Submitted: "Grace"},
		),
	)
	mustSetDocumentOwner(
		t,
		coordinator,
		"route",
		savedGreetingDocumentMetadata("ready", "Grace"),
	)
	snapshot := mustRemoveDocumentOwner(t, coordinator, "saved-editor")

	assertDocumentSnapshot(t, snapshot, serverBackedDocumentSnapshot{
		State:       savedGreetingDocumentMetadata("ready", "Grace"),
		ActiveOwner: "route",
		OwnerCount:  1,
	})
}

func mustSetDocumentOwner(
	t *testing.T,
	coordinator *serverBackedDocumentCoordinator,
	owner string,
	state serverBackedDocumentState,
) serverBackedDocumentSnapshot {
	t.Helper()
	snapshot, _, err := coordinator.Set(owner, state)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func mustRemoveDocumentOwner(
	t *testing.T,
	coordinator *serverBackedDocumentCoordinator,
	owner string,
) serverBackedDocumentSnapshot {
	t.Helper()
	snapshot, _, err := coordinator.Remove(owner)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertDocumentSnapshot(
	t *testing.T,
	got serverBackedDocumentSnapshot,
	want serverBackedDocumentSnapshot,
) {
	t.Helper()
	if got != want {
		t.Fatalf("document snapshot = %#v, want %#v", got, want)
	}
}
