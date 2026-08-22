package goframe

import (
	"reflect"
	"testing"
)

type stateParticipantMemoProps struct {
	ID int
}

type stateLifecycleOrderProbe struct {
	instance        *componentInstance
	slot            *stateSlot
	control         *resourceControl[string]
	commitSeen      bool
	rollbackSeen    bool
	stateReady      bool
	stateCleared    bool
	resourcePending bool
}

func (probe *stateLifecycleOrderProbe) finishRender(
	attempt *renderLifecycleAttempt,
	commit bool,
) {
	probe.resourcePending = probe.control != nil &&
		probe.control.pending.attempt == attempt &&
		!probe.control.committed
	if commit {
		probe.commitSeen = true
		probe.stateReady = len(probe.instance.stateSlots) == 2 &&
			probe.instance.stateSlots[0] == probe.slot &&
			probe.slot.owner == probe.instance &&
			probe.slot.pending == nil
		return
	}
	probe.rollbackSeen = true
	probe.stateCleared = probe.slot != nil &&
		probe.slot.value == nil &&
		probe.slot.owner == nil &&
		probe.slot.reducer == nil &&
		probe.slot.kind == "" &&
		probe.slot.pending == nil
}

func (props stateParticipantMemoProps) MemoEqual(next stateParticipantMemoProps) bool {
	return props.ID == next.ID
}

func TestUseStatePersistsWithinComponent(t *testing.T) {
	var value int
	var setValue func(int)
	instance := testComponentInstance("Counter", func() Node {
		value, setValue = UseState(0)
		return Text(ToString(value))
	}, nil)

	renderComponentInstance(instance)
	setValue(7)
	renderComponentInstance(instance)

	if value != 7 {
		t.Fatalf("state after component rerender = %d, want 7", value)
	}
}

func TestUseStateIsComponentScoped(t *testing.T) {
	var firstValue, secondValue int
	var setFirst func(int)
	first := testComponentInstance("First", func() Node {
		firstValue, setFirst = UseState(1)
		return Empty()
	}, nil)
	second := testComponentInstance("Second", func() Node {
		secondValue, _ = UseState(2)
		return Empty()
	}, nil)

	renderComponentInstance(first)
	renderComponentInstance(second)
	setFirst(10)
	renderComponentInstance(first)
	renderComponentInstance(second)

	if firstValue != 10 || secondValue != 2 {
		t.Fatalf("component states = %d, %d; want 10, 2", firstValue, secondValue)
	}
}

func TestUseStateSupportsMultipleSlots(t *testing.T) {
	var count int
	var label string
	var setCount func(int)
	var setLabel func(string)
	instance := testComponentInstance("Multi", func() Node {
		count, setCount = UseState(0)
		label, setLabel = UseState("first")
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	setCount(2)
	setLabel("second")
	renderComponentInstance(instance)

	if count != 2 || label != "second" {
		t.Fatalf("slots = %d, %q; want 2, second", count, label)
	}
}

func TestStateRenderParticipantStaysNilForHookFreeComponents(t *testing.T) {
	tests := []struct {
		name string
		new  func() *componentInstance
	}{
		{
			name: "plain rendering",
			new: func() *componentInstance {
				return testComponentInstance("PlainHookFree", func() Node {
					return Empty()
				}, nil)
			},
		},
		{
			name: "effect only",
			new: func() *componentInstance {
				return testComponentInstance("EffectHookFree", func() Node {
					UseEffect(func() Cleanup { return nil })
					return Empty()
				}, nil)
			},
		},
		{
			name: "unmount only",
			new: func() *componentInstance {
				return testComponentInstance("UnmountHookFree", func() Node {
					UseUnmount(func() {})
					return Empty()
				}, nil)
			},
		},
		{
			name: "context only",
			new: func() *componentInstance {
				ctx := CreateContext(1)
				return testComponentInstance("ContextHookFree", func() Node {
					_ = UseContext(ctx)
					return Empty()
				}, nil)
			},
		},
		{
			name: "memo only",
			new: func() *componentInstance {
				node := Component(
					"MemoHookFree",
					stateParticipantMemoProps{ID: 1},
					func(stateParticipantMemoProps) Node { return Empty() },
				).(ComponentNode)
				return newComponentInstance(node, "memo", nil, nil)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateLifecycleTestState(t)
			instance := test.new()
			for render := 0; render < 2; render++ {
				renderComponentInstance(instance)
				if instance.lifecycleAttempt.state != nil {
					t.Fatalf("hook-free render %d allocated state participant %p", render+1, instance.lifecycleAttempt.state)
				}
				flushPendingEffects()
			}
			deactivateComponent(instance)
			if instance.lifecycleAttempt.state != nil {
				t.Fatalf("hook-free deactivation retained state participant %p", instance.lifecycleAttempt.state)
			}
		})
	}
}

func TestStateRenderParticipantStaysNilAfterFailedAndProtectedHookFreeRenders(t *testing.T) {
	t.Run("failed render", func(t *testing.T) {
		errorsSeen := captureRuntimeErrors(t)
		instance := testComponentInstance("FailedHookFree", func() Node {
			panic("hook-free render failed")
		}, nil)

		renderComponentInstance(instance)
		if instance.lifecycleAttempt.state != nil {
			t.Fatalf("failed hook-free render allocated state participant %p", instance.lifecycleAttempt.state)
		}
		requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "FailedHookFree", "component render", "hook-free render failed")
	})

	t.Run("protected render", func(t *testing.T) {
		isolateLifecycleTestState(t)
		boundary := transactionTestBoundary()
		instance := testComponentInstance("ProtectedHookFree", func() Node {
			return Empty()
		}, nil)

		runProtectedSubtreeTestAttempt(boundary, func() {
			renderComponentInstance(instance)
			if !instance.lifecycleAttempt.active {
				t.Fatal("protected hook-free attempt committed before boundary result")
			}
			if instance.lifecycleAttempt.state != nil {
				t.Fatalf("protected hook-free render allocated state participant %p", instance.lifecycleAttempt.state)
			}
		})
		if instance.lifecycleAttempt.active || instance.lifecycleAttempt.state != nil {
			t.Fatalf("protected hook-free commit active=%v state=%p, want false/nil",
				instance.lifecycleAttempt.active, instance.lifecycleAttempt.state)
		}
		deactivateComponent(instance)
		deactivateComponent(boundary)
	})
}

func TestStateRenderParticipantAllocatesLazilyForStateHooks(t *testing.T) {
	tests := []struct {
		name                    string
		render                  func()
		wantGenericParticipants int
		wantResourceParticipant bool
	}{
		{
			name: "UseState",
			render: func() {
				_, _ = UseState(1)
			},
		},
		{
			name: "UseReducer",
			render: func() {
				_, _ = UseReducer(1, func(value, action int) int { return value + action })
			},
		},
		{
			name: "multiple state hooks",
			render: func() {
				_, _ = UseState(1)
				_, _ = UseReducer(2, func(value, action int) int { return value + action })
			},
		},
		{
			name: "UseResource",
			render: func() {
				_, _ = UseResource("resource", func(string, func(string), func(error)) Cleanup {
					return nil
				})
			},
			wantGenericParticipants: 1,
			wantResourceParticipant: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateLifecycleTestState(t)
			var participant *stateRenderParticipant
			var registrations int
			var resourceRegistered bool
			instance := testComponentInstance("Stateful", func() Node {
				test.render()
				participant = currentComponent.lifecycleAttempt.state
				registrations = len(currentComponent.lifecycleAttempt.participants)
				if registrations == 1 {
					_, resourceRegistered = currentComponent.lifecycleAttempt.participants[0].(*resourceControl[string])
				}
				return Empty()
			}, nil)

			renderComponentInstance(instance)
			if participant == nil || instance.lifecycleAttempt.state != participant {
				t.Fatalf("state hook participant captured=%p retained=%p, want one retained participant",
					participant, instance.lifecycleAttempt.state)
			}
			if registrations != test.wantGenericParticipants {
				t.Fatalf("generic render participants = %d, want %d", registrations, test.wantGenericParticipants)
			}
			if resourceRegistered != test.wantResourceParticipant {
				t.Fatalf("resource participant registered = %v, want %v",
					resourceRegistered, test.wantResourceParticipant)
			}
			if len(instance.lifecycleAttempt.participants) != 0 {
				t.Fatalf("finished attempt retained %d participant entries", len(instance.lifecycleAttempt.participants))
			}
			assertStateRenderParticipantCleared(t, participant)
			flushPendingEffects()
			deactivateComponent(instance)
			if instance.lifecycleAttempt.state != nil {
				t.Fatalf("deactivation retained state participant %p", instance.lifecycleAttempt.state)
			}
		})
	}
}

func TestStateRenderParticipantDoesNotImplementLifecycleParticipant(t *testing.T) {
	stateType := reflect.TypeOf((*stateRenderParticipant)(nil))
	participantType := reflect.TypeOf((*lifecycleRenderParticipant)(nil)).Elem()
	if stateType.Implements(participantType) {
		t.Fatalf("%v implements %v; state must use its dedicated lifecycle path",
			stateType, participantType)
	}
}

func TestMultipleStateHooksShareOneStateRenderParticipant(t *testing.T) {
	var first *stateRenderParticipant
	var second *stateRenderParticipant
	instance := testComponentInstance("SharedStateParticipant", func() Node {
		_, _ = UseState(1)
		first = currentComponent.lifecycleAttempt.state
		_, _ = UseReducer(2, func(value, action int) int { return value + action })
		second = currentComponent.lifecycleAttempt.state
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	if first == nil || second != first || instance.lifecycleAttempt.state != first {
		t.Fatalf("state participants first=%p second=%p retained=%p, want one shared participant",
			first, second, instance.lifecycleAttempt.state)
	}
	if len(instance.lifecycleAttempt.participants) != 0 {
		t.Fatalf("multiple state hooks retained %d generic participants, want 0",
			len(instance.lifecycleAttempt.participants))
	}
	assertStateRenderParticipantCleared(t, first)
}

func TestStateRenderParticipantFinishesBeforeGenericResourceParticipant(t *testing.T) {
	for _, test := range []struct {
		name string
		fail bool
	}{
		{name: "commit"},
		{name: "rollback", fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateLifecycleTestState(t)
			errorsSeen := captureRuntimeErrors(t)
			probe := &stateLifecycleOrderProbe{}
			var participantBacking []lifecycleRenderParticipant
			instance := testComponentInstance("StateResourceOrder", func() Node {
				_, _ = UseState(1)
				state := currentComponent.lifecycleAttempt.state
				probe.instance = currentComponent
				probe.slot = state.slots[0]
				currentComponent.lifecycleAttempt.participants = append(
					currentComponent.lifecycleAttempt.participants,
					probe,
				)
				_, _ = UseResource("resource", func(string, func(string), func(error)) Cleanup {
					return nil
				})
				probe.control, _ = state.slots[1].value.(*resourceControl[string])
				participantBacking = currentComponent.lifecycleAttempt.participants[:len(currentComponent.lifecycleAttempt.participants)]
				if test.fail {
					panic("state resource order rollback")
				}
				return Empty()
			}, nil)

			renderComponentInstance(instance)
			assertLifecycleParticipantBackingCleared(t, participantBacking)
			if !probe.resourcePending {
				t.Fatal("resource participant finished before the state ordering probe")
			}
			if test.fail {
				if !probe.rollbackSeen || !probe.stateCleared {
					t.Fatalf("rollback probe seen=%v state cleared=%v, want true/true",
						probe.rollbackSeen, probe.stateCleared)
				}
				if probe.control == nil || probe.control.pending.attempt != nil || probe.control.committed {
					t.Fatalf("resource after rollback control=%p pending=%p committed=%v, want non-nil/nil/false",
						probe.control, probe.control.pending.attempt, probe.control.committed)
				}
				requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "StateResourceOrder", "component render", "state resource order rollback")
				return
			}
			if !probe.commitSeen || !probe.stateReady {
				t.Fatalf("commit probe seen=%v state ready=%v, want true/true",
					probe.commitSeen, probe.stateReady)
			}
			if probe.control == nil || probe.control.pending.attempt != nil || !probe.control.committed {
				t.Fatalf("resource after commit control=%p pending=%p committed=%v, want non-nil/nil/true",
					probe.control, probe.control.pending.attempt, probe.control.committed)
			}
			deactivateComponent(instance)
		})
	}
}

func TestStateRenderParticipantReusesStorageAndClearsReferences(t *testing.T) {
	errorsSeen := captureRuntimeErrors(t)
	initial := true
	fail := false
	includeTrailing := false
	var participant *stateRenderParticipant
	var firstSlotBacking []*stateSlot
	var reducerBacking []stateReducerRenderUpdate
	var failedSlotBacking []*stateSlot
	var failedReducerBacking []stateReducerRenderUpdate
	var discarded *stateSlot
	instance := testComponentInstance("ReusableStateParticipant", func() Node {
		_, _ = UseState(1)
		_, _ = UseReducer(2, func(value, action int) int {
			return value + action
		})
		state := currentComponent.lifecycleAttempt.state
		if participant == nil {
			participant = state
		} else if state != participant {
			panic("state participant was not reused")
		}
		if initial {
			firstSlotBacking = state.slots[:len(state.slots)]
		}
		if !initial && !includeTrailing {
			reducerBacking = state.reducers[:len(state.reducers)]
		}
		if includeTrailing {
			_, setTrailing := UseState(3)
			discarded = state.slots[len(state.slots)-1]
			failedSlotBacking = state.slots[:len(state.slots)]
			failedReducerBacking = state.reducers[:len(state.reducers)]
			if fail {
				setTrailing(4)
				panic("state participant rollback")
			}
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	if instance.lifecycleAttempt.state != participant {
		t.Fatalf("first commit retained participant %p, want %p", instance.lifecycleAttempt.state, participant)
	}
	assertStateRenderParticipantCleared(t, participant)
	assertStateSlotBackingCleared(t, firstSlotBacking)

	initial = false
	renderComponentInstance(instance)
	if instance.lifecycleAttempt.state != participant {
		t.Fatalf("reducer-only update retained participant %p, want %p", instance.lifecycleAttempt.state, participant)
	}
	assertStateRenderParticipantCleared(t, participant)
	assertStateReducerBackingCleared(t, reducerBacking)

	includeTrailing = true
	fail = true
	renderComponentInstance(instance)
	if instance.lifecycleAttempt.state != participant {
		t.Fatalf("rollback retained participant %p, want %p", instance.lifecycleAttempt.state, participant)
	}
	assertStateRenderParticipantCleared(t, participant)
	assertStateSlotBackingCleared(t, failedSlotBacking)
	assertStateReducerBackingCleared(t, failedReducerBacking)
	if discarded == nil || discarded.owner != nil || discarded.pending != nil || discarded.value != nil || discarded.reducer != nil || discarded.kind != "" {
		t.Fatalf("discarded slot retained state: %#v", discarded)
	}

	fail = false
	renderComponentInstance(instance)
	if instance.lifecycleAttempt.state != participant || len(instance.stateSlots) != 3 {
		t.Fatalf("retry participant=%p slots=%d, want %p/3",
			instance.lifecycleAttempt.state, len(instance.stateSlots), participant)
	}
	assertStateRenderParticipantCleared(t, participant)
	requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "ReusableStateParticipant", "component render", "state participant rollback")
}

func TestStateRenderParticipantReusesReducerOnlyRenders(t *testing.T) {
	var first *stateRenderParticipant
	instance := testComponentInstance("ReducerOnlyParticipant", func() Node {
		_, _ = UseReducer(0, func(value, action int) int { return value + action })
		state := currentComponent.lifecycleAttempt.state
		if first == nil {
			first = state
		} else if state != first {
			panic("reducer-only participant was not reused")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	renderComponentInstance(instance)
	if instance.lifecycleAttempt.state != first {
		t.Fatalf("reducer-only participant = %p, want reused %p", instance.lifecycleAttempt.state, first)
	}
	assertStateRenderParticipantCleared(t, first)
}

func TestStateRenderParticipantReleaseDropsPointer(t *testing.T) {
	for _, test := range []struct {
		name    string
		release func(*componentInstance)
	}{
		{name: "release", release: releaseLifecycleRenderAttempt},
		{name: "deactivate", release: deactivateComponent},
	} {
		t.Run(test.name, func(t *testing.T) {
			instance := testComponentInstance("ReleasedParticipant", func() Node {
				_, _ = UseState(1)
				return Empty()
			}, nil)
			renderComponentInstance(instance)
			participant := instance.lifecycleAttempt.state
			if participant == nil {
				t.Fatal("stateful render did not allocate a participant")
			}

			test.release(instance)
			if instance.lifecycleAttempt.state != nil {
				t.Fatalf("lifecycle release retained state participant %p", instance.lifecycleAttempt.state)
			}
			assertStateRenderParticipantCleared(t, participant)
		})
	}
}

func assertStateRenderParticipantCleared(t *testing.T, state *stateRenderParticipant) {
	t.Helper()
	if state == nil {
		t.Fatal("state participant is nil")
	}
	if state.attempt != nil || state.instance != nil || len(state.slots) != 0 || len(state.values) != 0 || len(state.reducers) != 0 || state.dirty {
		t.Fatalf("state participant retained attempt=%p instance=%p slots=%d values=%d reducers=%d dirty=%v",
			state.attempt, state.instance, len(state.slots), len(state.values), len(state.reducers), state.dirty)
	}
}

func assertStateSlotBackingCleared(t *testing.T, slots []*stateSlot) {
	t.Helper()
	for index, slot := range slots {
		if slot != nil {
			t.Fatalf("state participant slot backing %d retained %p", index, slot)
		}
	}
}

func assertStateReducerBackingCleared(t *testing.T, reducers []stateReducerRenderUpdate) {
	t.Helper()
	for index, update := range reducers {
		if update.slot != nil || update.reducer != nil {
			t.Fatalf("state participant reducer backing %d retained slot=%p reducer=%T",
				index, update.slot, update.reducer)
		}
	}
}

func assertStateValueBackingCleared(t *testing.T, values []stateValueRenderUpdate) {
	t.Helper()
	for index, update := range values {
		if update.slot != nil || update.value != nil {
			t.Fatalf("state participant value backing %d retained slot=%p value=%T",
				index, update.slot, update.value)
		}
	}
}

func assertLifecycleParticipantBackingCleared(t *testing.T, participants []lifecycleRenderParticipant) {
	t.Helper()
	for index, participant := range participants {
		if participant != nil {
			t.Fatalf("lifecycle participant backing %d retained %T", index, participant)
		}
	}
}

func TestFailedInitialStateRenderLeavesNoGhostSlot(t *testing.T) {
	errorsSeen := captureRuntimeErrors(t)
	initial := 1
	fail := true
	observed := 0
	schedules := 0
	var failedSetter func(int)
	instance := testComponentInstance("TransactionalState", func() Node {
		observed, failedSetter = UseState(initial)
		if fail {
			panic("state render failed")
		}
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	if len(instance.stateSlots) != 0 {
		t.Errorf("state slots after failed initial render = %d, want 0", len(instance.stateSlots))
	}
	failedSetter(7)
	if instance.dirty || schedules != 0 {
		t.Errorf("discarded setter dirty=%v schedules=%d, want false/0", instance.dirty, schedules)
	}

	initial = 99
	fail = false
	renderComponentInstance(instance)
	if observed != 99 {
		t.Errorf("retry observed state = %d, want 99", observed)
	}
	if len(instance.stateSlots) != 1 || instance.stateSlots[0].value != 99 {
		t.Errorf("committed retry state slots = %#v, want one slot with 99", instance.stateSlots)
	}
	requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "TransactionalState", "component render", "state render failed")
}

func TestStateRenderValueNormalizationCommitsWithoutScheduling(t *testing.T) {
	schedules := 0
	normalized := 7
	var observed int
	var valueBacking []stateValueRenderUpdate
	instance := testComponentInstance("NormalizedState", func() Node {
		state := useStateSlot(10, "UseState")
		observed = stageStateValueForRender(state, normalized)
		participant := currentComponent.lifecycleAttempt.state
		valueBacking = participant.values[:len(participant.values)]
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	if observed != 7 || len(instance.stateSlots) != 1 || instance.stateSlots[0].value != 7 {
		t.Fatalf("initial normalized state observed=%d slots=%#v, want committed 7", observed, instance.stateSlots)
	}
	if schedules != 0 || instance.dirty {
		t.Fatalf("initial normalization schedules=%d dirty=%v, want 0/false", schedules, instance.dirty)
	}

	normalized = 3
	renderComponentInstance(instance)
	if observed != 3 || instance.stateSlots[0].value != 3 {
		t.Fatalf("existing normalized state observed=%d committed=%v, want 3/3", observed, instance.stateSlots[0].value)
	}
	if schedules != 0 || instance.dirty {
		t.Fatalf("existing normalization schedules=%d dirty=%v, want 0/false", schedules, instance.dirty)
	}
	assertStateValueBackingCleared(t, valueBacking)
}

func TestStateRenderValueNormalizationRollsBackAndRetryCommits(t *testing.T) {
	errorsSeen := captureRuntimeErrors(t)
	normalize := false
	fail := false
	var rendered int
	instance := testComponentInstance("TransactionalNormalization", func() Node {
		state := useStateSlot(10, "UseState")
		rendered = state.get()
		if normalize {
			rendered = stageStateValueForRender(state, 4)
			stageStateValueForRender(state, 2)
			rendered = 2
		}
		if fail {
			panic("normalized render failed")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	if instance.stateSlots[0].value != 10 {
		t.Fatalf("initial committed state = %v, want 10", instance.stateSlots[0].value)
	}

	normalize = true
	fail = true
	renderComponentInstance(instance)
	if rendered != 2 || instance.stateSlots[0].value != 10 {
		t.Fatalf("failed normalization rendered=%d committed=%v, want 2/10", rendered, instance.stateSlots[0].value)
	}

	fail = false
	renderComponentInstance(instance)
	if rendered != 2 || instance.stateSlots[0].value != 2 {
		t.Fatalf("retry normalization rendered=%d committed=%v, want 2/2", rendered, instance.stateSlots[0].value)
	}
	requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "TransactionalNormalization", "component render", "normalized render failed")
}

func TestPrivateStateRecordOutsideRenderDoesNotSchedule(t *testing.T) {
	schedules := 0
	var state stateHandle[int]
	instance := testComponentInstance("PrivateStateRecord", func() Node {
		state = useStateSlot(1, "UseState")
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	if !recordStateValueWithoutRender(state, 7) {
		t.Fatal("active committed state record was rejected")
	}
	if got := state.get(); got != 7 {
		t.Fatalf("recorded state = %d, want 7", got)
	}
	if instance.dirty || schedules != 0 {
		t.Fatalf("private record dirty=%v schedules=%d, want false/0", instance.dirty, schedules)
	}

	deactivateComponent(instance)
	if recordStateValueWithoutRender(state, 9) {
		t.Fatal("inactive state record succeeded")
	}
	if got := state.get(); got != 7 {
		t.Fatalf("inactive record changed state to %d, want 7", got)
	}
}

func TestStateRenderTransactionDiscardsMultipleSlotsAndCommitsRetryInOrder(t *testing.T) {
	errorsSeen := captureRuntimeErrors(t)
	countInitial := 1
	reducerInitial := 2
	labelInitial := "failed"
	fail := true
	schedules := 0
	reducerCalls := 0
	var failedSetter func(int)
	var failedDispatch func(int)
	var failedSlots []*stateSlot
	instance := testComponentInstance("TransactionalSlots", func() Node {
		_, failedSetter = UseState(countInitial)
		_, failedDispatch = UseReducer(reducerInitial, func(state, action int) int {
			reducerCalls++
			return state + action
		})
		_, _ = UseState(labelInitial)
		if fail {
			failedSlots = append(failedSlots[:0], currentComponent.lifecycleAttempt.state.slots...)
			panic("multiple state slots failed")
		}
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	if len(instance.stateSlots) != 0 {
		t.Fatalf("committed slots after failed multi-slot render = %d, want 0", len(instance.stateSlots))
	}
	if len(failedSlots) != 3 {
		t.Fatalf("speculative slots = %d, want 3", len(failedSlots))
	}
	for index, slot := range failedSlots {
		if slot.owner != nil || slot.pending != nil || slot.value != nil || slot.reducer != nil || slot.kind != "" {
			t.Errorf("discarded slot %d retained state: %#v", index, slot)
		}
	}
	failedSetter(9)
	failedDispatch(9)
	if instance.dirty || schedules != 0 || reducerCalls != 0 {
		t.Fatalf("discarded closures dirty=%v schedules=%d reducer calls=%d, want false/0/0",
			instance.dirty, schedules, reducerCalls)
	}

	countInitial = 10
	reducerInitial = 20
	labelInitial = "retry"
	fail = false
	renderComponentInstance(instance)
	if len(instance.stateSlots) != 3 {
		t.Fatalf("committed retry slots = %d, want 3", len(instance.stateSlots))
	}
	wantKinds := []string{"UseState", "UseReducer", "UseState"}
	wantValues := []any{10, 20, "retry"}
	for index, slot := range instance.stateSlots {
		if slot.kind != wantKinds[index] || slot.value != wantValues[index] || slot.owner != instance || slot.pending != nil {
			t.Errorf("committed retry slot %d = %#v, want kind %s value %v owner %p", index, slot, wantKinds[index], wantValues[index], instance)
		}
	}
	requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "TransactionalSlots", "component render", "multiple state slots failed")
}

func TestStateRenderTransactionPreservesCommittedPrefixAfterTrailingFailure(t *testing.T) {
	errorsSeen := captureRuntimeErrors(t)
	includeTrailing := false
	fail := false
	trailingInitial := 2
	instance := testComponentInstance("TransactionalPrefix", func() Node {
		_, _ = UseState(1)
		if includeTrailing {
			_, _ = UseState(trailingInitial)
		}
		if fail {
			panic("trailing slot failed")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	prefix := instance.stateSlots[0]
	includeTrailing = true
	fail = true
	renderComponentInstance(instance)
	if len(instance.stateSlots) != 1 || instance.stateSlots[0] != prefix || prefix.value != 1 {
		t.Fatalf("committed prefix after trailing failure = %#v, want unchanged slot %p value 1", instance.stateSlots, prefix)
	}

	trailingInitial = 3
	fail = false
	renderComponentInstance(instance)
	if len(instance.stateSlots) != 2 || instance.stateSlots[0] != prefix || instance.stateSlots[1].value != 3 {
		t.Fatalf("slots after trailing retry = %#v, want stable prefix and value 3", instance.stateSlots)
	}
	requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "TransactionalPrefix", "component render", "trailing slot failed")
}

func TestFailedInitialRenderTimeStateUpdatesLeaveNoSchedule(t *testing.T) {
	tests := []struct {
		name   string
		render func()
	}{
		{
			name: "state setter",
			render: func() {
				_, setValue := UseState(0)
				setValue(1)
			},
		},
		{
			name: "reducer dispatch",
			render: func() {
				_, dispatch := UseReducer(0, func(state, action int) int {
					return state + action
				})
				dispatch(1)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errorsSeen := captureRuntimeErrors(t)
			schedules := 0
			instance := testComponentInstance("FailedRenderUpdate", func() Node {
				test.render()
				panic("render-time update failed")
			}, func(*componentInstance) {
				schedules++
			})

			renderComponentInstance(instance)
			if len(instance.stateSlots) != 0 || instance.dirty || schedules != 0 {
				t.Fatalf("failed render-time update slots=%d dirty=%v schedules=%d, want 0/false/0",
					len(instance.stateSlots), instance.dirty, schedules)
			}
			requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "FailedRenderUpdate", "component render", "render-time update failed")
		})
	}
}

func TestPendingStateClosuresAreInvalidatedByLifecycleRelease(t *testing.T) {
	for _, test := range []struct {
		name    string
		release func(*componentInstance)
	}{
		{name: "release", release: releaseLifecycleRenderAttempt},
		{name: "deactivate", release: deactivateComponent},
	} {
		t.Run(test.name, func(t *testing.T) {
			schedules := 0
			instance := testComponentInstance("PendingState", func() Node { return Empty() }, func(*componentInstance) {
				schedules++
			})
			clearComponentDirty(instance)
			beginLifecycleRenderAttempt(instance)
			previous := currentComponent
			currentComponent = instance
			instance.stateIndex = 0
			state := useStateSlot(1, "UseState")
			reducer := useStateSlot(2, "UseReducer")
			reducerCalls := 0
			setReducer(reducer.slot, Reducer[int, int](func(value, action int) int {
				reducerCalls++
				return value + action
			}))
			currentComponent = previous
			slots := []*stateSlot{state.slot, reducer.slot}

			test.release(instance)
			for index, slot := range slots {
				if slot.owner != nil || slot.pending != nil || slot.value != nil || slot.reducer != nil || slot.kind != "" {
					t.Fatalf("released speculative slot %d retained state: %#v", index, slot)
				}
			}
			if instance.lifecycleAttempt.active || len(instance.lifecycleAttempt.participants) != 0 || instance.lifecycleAttempt.state != nil {
				t.Fatalf("released lifecycle attempt retained state: %#v", instance.lifecycleAttempt)
			}
			state.set(2)
			dispatchReducer[int, int](reducer.slot, 1)
			if instance.dirty || schedules != 0 || reducerCalls != 0 {
				t.Fatalf("released closures dirty=%v schedules=%d reducer calls=%d, want false/0/0",
					instance.dirty, schedules, reducerCalls)
			}
		})
	}
}

func TestUseStateOutsideComponentPanics(t *testing.T) {
	currentComponent = nil
	defer func() {
		if recovered := comparablePanicValue(recover()); recovered != "goframe: UseState must be called during component render" {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	UseState(0)
}

func TestStateMarksOwnerDirtyAndSchedulesIt(t *testing.T) {
	var setState func(int)
	var scheduled *componentInstance
	instance := testComponentInstance("Owner", func() Node {
		_, setState = UseState(0)
		return Empty()
	}, func(instance *componentInstance) {
		scheduled = instance
	})
	renderComponentInstance(instance)

	setState(1)

	if !instance.dirty || scheduled != instance {
		t.Fatalf("dirty=%v scheduled=%p owner=%p", instance.dirty, scheduled, instance)
	}
}

func TestStateSetSameComparableValueDoesNotSchedule(t *testing.T) {
	var setState func(string)
	schedules := 0
	instance := testComponentInstance("Owner", func() Node {
		_, setState = UseState("same")
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)

	setState("same")

	if instance.dirty || schedules != 0 {
		t.Fatalf("dirty=%v schedules=%d, want no-op", instance.dirty, schedules)
	}
}

func TestDirtyChildDoesNotMarkRootOrSibling(t *testing.T) {
	var setState func(int)
	rootSchedules := 0
	childSchedules := 0
	siblingSchedules := 0
	root := testComponentInstance("Root", func() Node { return Empty() }, func(*componentInstance) {
		rootSchedules++
	})
	child := testComponentInstance("Child", func() Node {
		_, setState = UseState(0)
		return Empty()
	}, func(*componentInstance) {
		childSchedules++
	})
	sibling := testComponentInstance("Sibling", func() Node { return Empty() }, func(*componentInstance) {
		siblingSchedules++
	})
	renderComponentInstance(root)
	renderComponentInstance(child)
	renderComponentInstance(sibling)

	setState(1)

	if root.dirty || sibling.dirty || rootSchedules != 0 || siblingSchedules != 0 {
		t.Fatalf("root dirty=%v schedules=%d, sibling dirty=%v schedules=%d",
			root.dirty, rootSchedules, sibling.dirty, siblingSchedules)
	}
	if !child.dirty || childSchedules != 1 {
		t.Fatalf("child dirty=%v schedules=%d, want dirty scheduled child", child.dirty, childSchedules)
	}
}

func TestStateSetDuringRenderRemainsDirty(t *testing.T) {
	schedules := 0
	instance := testComponentInstance("RenderSet", func() Node {
		value, setValue := UseState(0)
		if value == 0 {
			setValue(1)
		}
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)

	if !instance.dirty || schedules != 1 || instance.stateSlots[0].value != 1 {
		t.Fatalf("dirty=%v schedules=%d value=%v, want dirty scheduled component with 1",
			instance.dirty, schedules, instance.stateSlots[0].value)
	}
}

func TestUnmountedStateDoesNotSchedule(t *testing.T) {
	var setState func(int)
	schedules := 0
	instance := testComponentInstance("Unmounted", func() Node {
		_, setState = UseState(0)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)
	deactivateComponent(instance)

	setState(1)

	if schedules != 0 || instance.dirty {
		t.Fatalf("schedules=%d dirty=%v, want inactive", schedules, instance.dirty)
	}
}

func TestUseReducerReturnsInitialState(t *testing.T) {
	var value int
	instance := testComponentInstance("Reducer", func() Node {
		value, _ = UseReducer(3, func(state int, action int) int {
			return state + action
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)

	if value != 3 {
		t.Fatalf("reducer initial state = %d, want 3", value)
	}
}

func TestUseReducerDispatchAppliesToLatestState(t *testing.T) {
	var value int
	var dispatch func(int)
	instance := testComponentInstance("Reducer", func() Node {
		value, dispatch = UseReducer(0, func(state int, action int) int {
			return state + action
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	dispatch(1)
	dispatch(1)
	renderComponentInstance(instance)

	if value != 2 {
		t.Fatalf("reducer state = %d, want 2", value)
	}
}

func TestUseReducerStaleDispatchUsesLatestState(t *testing.T) {
	var value int
	var firstDispatch func(int)
	var latestDispatch func(int)
	instance := testComponentInstance("Reducer", func() Node {
		value, latestDispatch = UseReducer(0, func(state int, action int) int {
			return state + action
		})
		if firstDispatch == nil {
			firstDispatch = latestDispatch
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	latestDispatch(5)
	renderComponentInstance(instance)
	firstDispatch(7)
	renderComponentInstance(instance)

	if value != 12 {
		t.Fatalf("stale dispatch reducer state = %d, want 12", value)
	}
}

func TestUseReducerStaleDispatchUsesLatestReducer(t *testing.T) {
	var value int
	var firstDispatch func(int)
	var currentDispatch func(int)
	useMultiplier := false
	instance := testComponentInstance("Reducer", func() Node {
		if useMultiplier {
			value, currentDispatch = UseReducer(0, func(state int, action int) int {
				return state + action*10
			})
		} else {
			value, currentDispatch = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		if firstDispatch == nil {
			firstDispatch = currentDispatch
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	currentDispatch(1)
	renderComponentInstance(instance)
	useMultiplier = true
	renderComponentInstance(instance)
	firstDispatch(1)
	renderComponentInstance(instance)

	if value != 11 {
		t.Fatalf("stale dispatch used wrong reducer, state = %d, want 11", value)
	}
}

func TestFailedReducerRenderKeepsCommittedReducer(t *testing.T) {
	errorsSeen := captureRuntimeErrors(t)
	useCandidate := false
	fail := false
	var firstDispatch func(int)
	instance := testComponentInstance("TransactionalReducer", func() Node {
		reducer := Reducer[int, int](func(state, action int) int {
			return state + action
		})
		if useCandidate {
			reducer = func(state, action int) int {
				return state + action*100
			}
		}
		_, dispatch := UseReducer(1, reducer)
		if firstDispatch == nil {
			firstDispatch = dispatch
		}
		if fail {
			panic("reducer render failed")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	slot := instance.stateSlots[0]
	committedReducer := reflect.ValueOf(slot.reducer).Pointer()

	useCandidate = true
	fail = true
	renderComponentInstance(instance)
	if instance.stateSlots[0] != slot {
		t.Fatal("failed reducer render replaced committed slot pointer")
	}
	if got := reflect.ValueOf(slot.reducer).Pointer(); got != committedReducer {
		t.Errorf("failed reducer render changed reducer pointer = %x, want %x", got, committedReducer)
	}
	firstDispatch(1)
	if slot.value != 2 {
		t.Errorf("old dispatch after failed render produced %v, want 2", slot.value)
	}

	fail = false
	renderComponentInstance(instance)
	firstDispatch(1)
	if slot.value != 102 {
		t.Errorf("old dispatch after successful render produced %v, want 102", slot.value)
	}
	requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "TransactionalReducer", "component render", "reducer render failed")
}

func TestUseReducerStateTypeMismatchPanics(t *testing.T) {
	useString := false
	instance := testComponentInstance("Reducer", func() Node {
		if useString {
			_, _ = UseReducer("", func(state string, action string) string {
				return state + action
			})
		} else {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	slot := instance.stateSlots[0]
	reducer := reflect.ValueOf(slot.reducer).Pointer()
	useString = true

	assertPanic(t, "goframe: UseReducer state type changed between component renders", func() {
		renderComponentInstance(instance)
	})
	if len(instance.stateSlots) != 1 || instance.stateSlots[0] != slot || reflect.ValueOf(slot.reducer).Pointer() != reducer {
		t.Fatal("state type mismatch changed committed reducer slot")
	}
}

func TestStateHookKindMismatchPanics(t *testing.T) {
	useReducer := false
	instance := testComponentInstance("StateKind", func() Node {
		if useReducer {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		} else {
			_, _ = UseState(0)
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	slot := instance.stateSlots[0]
	useReducer = true

	assertPanic(t, "goframe: hook at state slot 0 changed from UseState to UseReducer", func() {
		renderComponentInstance(instance)
	})
	if len(instance.stateSlots) != 1 || instance.stateSlots[0] != slot || slot.kind != "UseState" {
		t.Fatal("state hook-kind mismatch changed committed slot")
	}
}

func TestReducerHookKindMismatchPanics(t *testing.T) {
	useState := false
	instance := testComponentInstance("ReducerKind", func() Node {
		if useState {
			_, _ = UseState(0)
		} else {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	useState = true

	assertPanic(t, "goframe: hook at state slot 0 changed from UseReducer to UseState", func() {
		renderComponentInstance(instance)
	})
}

func TestUseReducerReducerTypeMismatchPanics(t *testing.T) {
	useStringAction := false
	instance := testComponentInstance("Reducer", func() Node {
		if useStringAction {
			_, _ = UseReducer(0, func(state int, action string) int {
				if action == "" {
					return state
				}
				return state + 1
			})
		} else {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	slot := instance.stateSlots[0]
	reducer := reflect.ValueOf(slot.reducer).Pointer()
	useStringAction = true

	assertPanic(t, "goframe: UseReducer reducer type changed between component renders", func() {
		renderComponentInstance(instance)
	})
	if len(instance.stateSlots) != 1 || instance.stateSlots[0] != slot || reflect.ValueOf(slot.reducer).Pointer() != reducer {
		t.Fatal("reducer type mismatch changed committed reducer slot")
	}
}

func TestUseReducerOutsideComponentPanics(t *testing.T) {
	currentComponent = nil
	assertPanic(t, "goframe: UseReducer must be called during component render", func() {
		UseReducer(0, func(state int, action int) int {
			return state + action
		})
	})
}

func TestUseReducerNilReducerPanics(t *testing.T) {
	instance := testComponentInstance("Reducer", func() Node {
		_, _ = UseReducer[int, int](0, nil)
		return Empty()
	}, nil)

	assertPanic(t, "goframe: UseReducer reducer must not be nil", func() {
		renderComponentInstance(instance)
	})
}

func TestUseReducerDispatchAfterUnmountDoesNotSchedule(t *testing.T) {
	var dispatch func(int)
	reducerCalls := 0
	schedules := 0
	instance := testComponentInstance("Reducer", func() Node {
		_, dispatch = UseReducer(0, func(state int, action int) int {
			reducerCalls++
			return state + action
		})
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)
	deactivateComponent(instance)

	dispatch(1)

	if schedules != 0 || reducerCalls != 0 || instance.dirty {
		t.Fatalf("schedules=%d reducerCalls=%d dirty=%v, want inactive no-op", schedules, reducerCalls, instance.dirty)
	}
}

func TestUseReducerDispatchDuringRenderRemainsDirty(t *testing.T) {
	schedules := 0
	instance := testComponentInstance("Reducer", func() Node {
		value, dispatch := UseReducer(0, func(state int, action int) int {
			return state + action
		})
		if value == 0 {
			dispatch(1)
		}
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)

	if !instance.dirty || schedules != 1 || instance.stateSlots[0].value != 1 {
		t.Fatalf("dirty=%v schedules=%d value=%v, want dirty scheduled component with 1",
			instance.dirty, schedules, instance.stateSlots[0].value)
	}
}

func TestUseReducerDispatchSamePrimitiveValueDoesNotSchedule(t *testing.T) {
	var dispatch func(int)
	schedules := 0
	instance := testComponentInstance("Reducer", func() Node {
		_, dispatch = UseReducer(1, func(state int, action int) int {
			return state
		})
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)

	dispatch(1)

	if instance.dirty || schedules != 0 {
		t.Fatalf("dirty=%v schedules=%d, want no-op", instance.dirty, schedules)
	}
}

func TestUpdateBatchCoalescesPendingRequests(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}
	updates := 0

	batch.request(enqueue, func() { updates++ })
	batch.request(enqueue, func() { updates++ })
	batch.request(enqueue, func() { updates++ })

	if len(queued) != 1 {
		t.Fatalf("queued updates = %d, want 1", len(queued))
	}
	if updates != 0 {
		t.Fatalf("updates before queued callback = %d, want 0", updates)
	}
	queued[0]()
	if updates != 1 {
		t.Fatalf("updates = %d, want 1", updates)
	}
}

func TestUpdateBatchAllowsRequestAfterFlush(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}
	updates := 0

	batch.request(enqueue, func() { updates++ })
	queued[0]()

	batch.request(enqueue, func() { updates++ })
	if len(queued) != 2 {
		t.Fatalf("queued updates after flush = %d, want 2", len(queued))
	}
	queued[1]()
	if updates != 2 {
		t.Fatalf("updates = %d, want 2", updates)
	}
}

func TestUpdateBatchResetClearsPendingRequest(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}

	batch.request(enqueue, func() {})
	batch.reset()
	batch.request(enqueue, func() {})

	if len(queued) != 2 {
		t.Fatalf("queued updates after reset = %d, want 2", len(queued))
	}
}

func TestUpdateBatchResetInvalidatesQueuedRequest(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}
	updateA := 0
	updateB := 0

	batch.request(enqueue, func() { updateA++ })
	if len(queued) != 1 {
		t.Fatalf("queued updates = %d, want 1", len(queued))
	}
	if updateA != 0 {
		t.Fatalf("updateA before queued callback = %d, want 0", updateA)
	}

	batch.reset()
	batch.request(enqueue, func() { updateB++ })
	if len(queued) != 2 {
		t.Fatalf("queued updates after reset = %d, want 2", len(queued))
	}

	queued[0]()
	if updateA != 0 {
		t.Fatalf("stale updateA ran %d time(s), want 0", updateA)
	}
	if updateB != 0 {
		t.Fatalf("updateB before current callback = %d, want 0", updateB)
	}

	queued[1]()
	if updateB != 1 {
		t.Fatalf("updateB = %d, want 1", updateB)
	}
	if len(queued) != 2 {
		t.Fatalf("queued updates after callbacks = %d, want 2", len(queued))
	}
}

func testComponentInstance(name string, render func() Node, schedule func(*componentInstance)) *componentInstance {
	node := Component(name, struct{}{}, func(struct{}) Node {
		return render()
	}).(ComponentNode)
	return newComponentInstance(node, "", nil, schedule)
}

func assertPanic(t *testing.T, want any, fn func()) {
	t.Helper()
	defer func() {
		if recovered := comparablePanicValue(recover()); recovered != want {
			t.Fatalf("panic = %v, want %v", recovered, want)
		}
	}()
	fn()
}
