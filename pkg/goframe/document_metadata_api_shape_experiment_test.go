//go:build goframe_document_state_experiment

package goframe

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDocumentMetadataAPIShapeFirstRenderPublication(t *testing.T) {
	tests := []struct {
		name       string
		render     func(documentMetadataAPIShapeTestValue) func() Node
		stateSlots int
	}{
		{
			name: "hook",
			render: func(value documentMetadataAPIShapeTestValue) func() Node {
				return func() Node {
					UseDocumentMetadata(value.public())
					return Empty()
				}
			},
			stateSlots: 1,
		},
		{
			name: "component",
			render: func(value documentMetadataAPIShapeTestValue) func() Node {
				return func() Node {
					return DocumentMetadataComponent(DocumentMetadataComponentProps{
						Metadata: value.public(),
						Children: []Node{Text("child")},
					})
				}
			},
			stateSlots: 1,
		},
		{
			name: "handle",
			render: func(value documentMetadataAPIShapeTestValue) func() Node {
				return func() Node {
					owner := UseDocumentMetadataOwner()
					UseOwnedDocumentMetadata(owner, value.public())
					return Empty()
				}
			},
			stateSlots: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
			value := documentMetadataAPIShapeTestValue{"A", "Description A"}
			coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
			renders := 0
			render := test.render(value)
			instance := testComponentInstance("APIShape"+test.name, func() Node {
				renders++
				return render()
			}, nil)

			coordinator.beginUpdate()
			renderComponentInstance(instance)
			coordinator.commitUpdate()

			if renders != 1 {
				t.Fatalf("committed renders = %d, want 1", renders)
			}
			if len(instance.stateSlots) != test.stateSlots {
				t.Fatalf("state slots = %d, want %d", len(instance.stateSlots), test.stateSlots)
			}
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, value, 1)
			if coordinator.statistics.tokenCreations != 1 ||
				coordinator.statistics.committedIDAssignments != 1 ||
				coordinator.statistics.documentPublications != 1 {
				t.Fatalf("first render statistics = %#v", coordinator.statistics)
			}
			if !reflect.DeepEqual(*publications, []documentMetadataValue{value.private()}) {
				t.Fatalf("publications = %#v", *publications)
			}
		})
	}
}

func TestDocumentMetadataAPIShapeFailedInitialRenderCommitsNothing(t *testing.T) {
	tests := []struct {
		name   string
		render func(DocumentMetadata) Node
	}{
		{
			name: "hook",
			render: func(value DocumentMetadata) Node {
				UseDocumentMetadata(value)
				panic("failed hook render")
			},
		},
		{
			name: "component",
			render: func(value DocumentMetadata) Node {
				DocumentMetadataComponent(DocumentMetadataComponentProps{Metadata: value})
				panic("failed component render")
			},
		},
		{
			name: "handle",
			render: func(value DocumentMetadata) Node {
				owner := UseDocumentMetadataOwner()
				UseOwnedDocumentMetadata(owner, value)
				panic("failed handle render")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
			value := documentMetadataAPIShapeTestValue{"Failed", "Must not publish"}
			coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
			var runtimeErrors []ErrorInfo
			restoreErrors := SetErrorHandler(func(info ErrorInfo) {
				runtimeErrors = append(runtimeErrors, info)
			})
			t.Cleanup(restoreErrors)
			instance := testComponentInstance("FailedAPIShape"+test.name, func() Node {
				return test.render(value.public())
			}, nil)

			coordinator.beginUpdate()
			renderComponentInstance(instance)
			coordinator.commitUpdate()

			if len(instance.stateSlots) != 0 {
				t.Fatalf("failed render state slots = %d, want 0", len(instance.stateSlots))
			}
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)
			if coordinator.statistics.tokenCreations != 0 ||
				coordinator.statistics.committedIDAssignments != 0 ||
				coordinator.statistics.documentPublications != 0 {
				t.Fatalf("failed render statistics = %#v", coordinator.statistics)
			}
			if len(*publications) != 0 {
				t.Fatalf("failed render publications = %#v", *publications)
			}
			if len(runtimeErrors) != 1 || !strings.Contains(runtimeErrors[0].Panic.(string), "failed ") {
				t.Fatalf("failed render errors = %#v", runtimeErrors)
			}
		})
	}
}

func TestDocumentMetadataAPIShapeDirectReplacement(t *testing.T) {
	tests := []struct {
		name   string
		render func(DocumentMetadata) func() Node
	}{
		{
			name: "hook",
			render: func(value DocumentMetadata) func() Node {
				return func() Node {
					UseDocumentMetadata(value)
					return Empty()
				}
			},
		},
		{
			name: "component",
			render: func(value DocumentMetadata) func() Node {
				return func() Node {
					return DocumentMetadataComponent(DocumentMetadataComponentProps{Metadata: value})
				}
			},
		},
		{
			name: "handle",
			render: func(value DocumentMetadata) func() Node {
				return func() Node {
					owner := UseDocumentMetadataOwner()
					UseOwnedDocumentMetadata(owner, value)
					return Empty()
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
			valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}
			valueB := documentMetadataAPIShapeTestValue{"B", "Description B"}
			coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
			ownerA := testComponentInstance("APIShapeA", test.render(valueA.public()), nil)
			ownerB := testComponentInstance("APIShapeB", test.render(valueB.public()), nil)

			coordinator.beginUpdate()
			renderComponentInstance(ownerA)
			coordinator.commitUpdate()

			coordinator.beginUpdate()
			renderComponentInstance(ownerB)
			deactivateComponent(ownerA)
			coordinator.commitUpdate()

			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB, 1)
			if !reflect.DeepEqual(*publications, []documentMetadataValue{
				valueA.private(),
				valueB.private(),
			}) {
				t.Fatalf("replacement publications = %#v, want A -> B", *publications)
			}
			if coordinator.statistics.baselineRestorations != 0 ||
				coordinator.statistics.releases != 1 {
				t.Fatalf("replacement statistics = %#v", coordinator.statistics)
			}
		})
	}
}

func TestDocumentMetadataAPIShapePriorityUpdateReleaseAndRemount(t *testing.T) {
	tests := []struct {
		name   string
		render func(*DocumentMetadataOwner, DocumentMetadata) (func() Node, **DocumentMetadataOwner)
	}{
		{
			name: "hook",
			render: func(_ *DocumentMetadataOwner, value DocumentMetadata) (func() Node, **DocumentMetadataOwner) {
				return func() Node {
					UseDocumentMetadata(value)
					return Empty()
				}, nil
			},
		},
		{
			name: "component",
			render: func(_ *DocumentMetadataOwner, value DocumentMetadata) (func() Node, **DocumentMetadataOwner) {
				return func() Node {
					return DocumentMetadataComponent(DocumentMetadataComponentProps{Metadata: value})
				}, nil
			},
		},
		{
			name: "handle",
			render: func(_ *DocumentMetadataOwner, value DocumentMetadata) (func() Node, **DocumentMetadataOwner) {
				var owner *DocumentMetadataOwner
				return func() Node {
					owner = UseDocumentMetadataOwner()
					UseOwnedDocumentMetadata(owner, value)
					return Empty()
				}, &owner
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
			valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}
			valueA2 := documentMetadataAPIShapeTestValue{"A2", "Description A2"}
			valueB := documentMetadataAPIShapeTestValue{"B", "Description B"}
			valueB2 := documentMetadataAPIShapeTestValue{"B2", "Description B2"}
			coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
			renderA, handleA := test.render(nil, valueA.public())
			renderB, handleB := test.render(nil, valueB.public())
			ownerA := testComponentInstance("PriorityA", renderA, nil)
			ownerB := testComponentInstance("PriorityB", renderB, nil)

			coordinator.beginUpdate()
			renderComponentInstance(ownerA)
			renderComponentInstance(ownerB)
			coordinator.commitUpdate()
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB, 2)
			if got := coordinator.ownerIDs(); !reflect.DeepEqual(got, []uint64{1, 2}) {
				t.Fatalf("initial owner order = %#v, want [1 2]", got)
			}

			updatedA, _ := test.render(nil, valueA2.public())
			ownerA.node.render = updatedA
			beforePublications := len(*publications)
			coordinator.beginUpdate()
			renderComponentInstance(ownerA)
			coordinator.commitUpdate()
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB, 2)
			if len(*publications) != beforePublications ||
				!reflect.DeepEqual(coordinator.ownerIDs(), []uint64{1, 2}) {
				t.Fatalf("non-selected update publications=%#v owners=%#v", *publications, coordinator.ownerIDs())
			}

			coordinator.beginUpdate()
			deactivateComponent(ownerA)
			coordinator.commitUpdate()
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB, 1)
			if len(*publications) != beforePublications {
				t.Fatalf("non-selected release publications = %#v", *publications)
			}

			updatedB, _ := test.render(nil, valueB2.public())
			ownerB.node.render = updatedB
			coordinator.beginUpdate()
			renderComponentInstance(ownerB)
			coordinator.commitUpdate()
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB2, 1)
			if got := coordinator.ownerIDs(); !reflect.DeepEqual(got, []uint64{2}) {
				t.Fatalf("selected update owner IDs = %#v, want [2]", got)
			}
			if handleA != nil && (*handleA).ID() != 1 {
				t.Fatalf("handle A ID = %d, want 1", (*handleA).ID())
			}
			if handleB != nil && (*handleB).ID() != 2 {
				t.Fatalf("handle B ID = %d, want 2", (*handleB).ID())
			}

			coordinator.beginUpdate()
			deactivateComponent(ownerB)
			coordinator.commitUpdate()
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)

			renderRemount, remountedHandle := test.render(nil, valueA.public())
			remounted := testComponentInstance("PriorityRemount", renderRemount, nil)
			coordinator.beginUpdate()
			renderComponentInstance(remounted)
			coordinator.commitUpdate()
			assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
			if got := coordinator.ownerIDs(); !reflect.DeepEqual(got, []uint64{3}) {
				t.Fatalf("remount owner IDs = %#v, want [3]", got)
			}
			if remountedHandle != nil && (*remountedHandle).ID() != 3 {
				t.Fatalf("remounted handle ID = %d, want 3", (*remountedHandle).ID())
			}
		})
	}
}

func TestDocumentMetadataAPIShapeHookSlotsRemainOrdered(t *testing.T) {
	baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
	valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}
	valueB := documentMetadataAPIShapeTestValue{"B", "Description B"}
	coordinator, _ := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
	instance := testComponentInstance("TwoHookSlots", func() Node {
		UseDocumentMetadata(valueA.public())
		UseDocumentMetadata(valueB.public())
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(instance)
	coordinator.commitUpdate()

	if got := coordinator.ownerIDs(); !reflect.DeepEqual(got, []uint64{1, 2}) {
		t.Fatalf("hook owner IDs = %#v, want [1 2]", got)
	}
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB, 2)

	coordinator.beginUpdate()
	deactivateComponent(instance)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)
	if coordinator.statistics.releases != 2 {
		t.Fatalf("hook releases = %d, want 2", coordinator.statistics.releases)
	}
}

func TestDocumentMetadataAPIShapeComponentPreservesChildren(t *testing.T) {
	baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
	value := documentMetadataAPIShapeTestValue{"A", "Description A"}
	coordinator, _ := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
	child := El("span", Props{"data-testid": "child"}, Text("child"))
	instance := testComponentInstance("MetadataComponent", func() Node {
		return DocumentMetadataComponent(DocumentMetadataComponentProps{
			Metadata: value.public(),
			Children: []Node{child},
		})
	}, nil)

	coordinator.beginUpdate()
	rendered := renderComponentInstance(instance)
	coordinator.commitUpdate()

	fragment, ok := rendered.(FragmentNode)
	if !ok || len(fragment.Children) != 1 || !reflect.DeepEqual(fragment.Children[0], child) {
		t.Fatalf("component output = %#v, want the original child in one fragment", rendered)
	}
}

func TestDocumentMetadataAPIShapeHandleDuplicateContract(t *testing.T) {
	baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
	valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}
	valueB := documentMetadataAPIShapeTestValue{"B", "Description B"}
	coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
	var handle *DocumentMetadataOwner
	primary := testComponentInstance("HandlePrimary", func() Node {
		handle = UseDocumentMetadataOwner()
		useOwnedDocumentMetadataAPIShapeTestHelper(handle, valueA.public())
		return Empty()
	}, nil)
	duplicate := testComponentInstance("HandleDuplicate", func() Node {
		UseOwnedDocumentMetadata(handle, valueA.public())
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(primary)
	renderComponentInstance(duplicate)
	coordinator.commitUpdate()

	if handle == nil || handle.ActivePublications() != 2 || handle.ID() != 1 {
		t.Fatalf("duplicated handle = %#v", handle)
	}
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
	if coordinator.statistics.activeAdditions != 1 || len(*publications) != 1 {
		t.Fatalf("duplicate activation statistics=%#v publications=%#v", coordinator.statistics, *publications)
	}

	coordinator.beginUpdate()
	deactivateComponent(duplicate)
	coordinator.commitUpdate()
	if handle.ActivePublications() != 1 || coordinator.statistics.releases != 0 || len(*publications) != 1 {
		t.Fatalf("duplicate release handle=%#v statistics=%#v publications=%#v", handle, coordinator.statistics, *publications)
	}

	primary.node.render = func() Node {
		stable := UseDocumentMetadataOwner()
		if stable != handle {
			panic("handle identity changed")
		}
		UseOwnedDocumentMetadata(stable, valueB.public())
		return Empty()
	}
	coordinator.beginUpdate()
	renderComponentInstance(primary)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB, 1)
	if handle.ID() != 1 || coordinator.statistics.updates != 1 {
		t.Fatalf("sole-primary update handle=%#v statistics=%#v", handle, coordinator.statistics)
	}

	coordinator.beginUpdate()
	deactivateComponent(primary)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)
	if coordinator.statistics.releases != 1 {
		t.Fatalf("final handle releases = %d, want 1", coordinator.statistics.releases)
	}
}

func TestDocumentMetadataAPIShapeHandleRejectsConflictingPublications(t *testing.T) {
	baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
	valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}
	valueB := documentMetadataAPIShapeTestValue{"B", "Description B"}
	coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
	var runtimeErrors []ErrorInfo
	restoreErrors := SetErrorHandler(func(info ErrorInfo) {
		runtimeErrors = append(runtimeErrors, info)
	})
	t.Cleanup(restoreErrors)
	instance := testComponentInstance("ConflictingHandle", func() Node {
		owner := UseDocumentMetadataOwner()
		UseOwnedDocumentMetadata(owner, valueA.public())
		UseOwnedDocumentMetadata(owner, valueA.public())
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(instance)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)

	instance.node.render = func() Node {
		owner := UseDocumentMetadataOwner()
		UseOwnedDocumentMetadata(owner, valueB.public())
		UseOwnedDocumentMetadata(owner, valueA.public())
		return Empty()
	}
	coordinator.beginUpdate()
	renderComponentInstance(instance)
	coordinator.commitUpdate()

	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
	if len(*publications) != 1 {
		t.Fatalf("conflict publications = %#v", *publications)
	}
	if len(runtimeErrors) != 1 ||
		!strings.Contains(runtimeErrors[0].Panic.(string), "conflicting active publications") {
		t.Fatalf("conflict runtime errors = %#v", runtimeErrors)
	}
}

func TestDocumentMetadataAPIShapeHandleDoesNotTransferPrimary(t *testing.T) {
	baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
	valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}
	valueB := documentMetadataAPIShapeTestValue{"B", "Description B"}
	coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
	var runtimeErrors []ErrorInfo
	restoreErrors := SetErrorHandler(func(info ErrorInfo) {
		runtimeErrors = append(runtimeErrors, info)
	})
	t.Cleanup(restoreErrors)
	var handle *DocumentMetadataOwner
	primary := testComponentInstance("PrimaryPublication", func() Node {
		handle = UseDocumentMetadataOwner()
		UseOwnedDocumentMetadata(handle, valueA.public())
		return Empty()
	}, nil)
	duplicateValue := valueA
	duplicate := testComponentInstance("DuplicatePublication", func() Node {
		UseOwnedDocumentMetadata(handle, duplicateValue.public())
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(primary)
	renderComponentInstance(duplicate)
	coordinator.commitUpdate()

	coordinator.beginUpdate()
	deactivateComponent(primary)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
	if handle.ActivePublications() != 1 {
		t.Fatalf("publications after primary release = %d, want 1", handle.ActivePublications())
	}

	duplicateValue = valueB
	coordinator.beginUpdate()
	renderComponentInstance(duplicate)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
	if len(runtimeErrors) != 1 ||
		!strings.Contains(runtimeErrors[0].Panic.(string), "conflicting active publications") {
		t.Fatalf("non-primary update errors = %#v", runtimeErrors)
	}
	if len(*publications) != 1 {
		t.Fatalf("non-primary update publications = %#v", *publications)
	}

	coordinator.beginUpdate()
	deactivateComponent(duplicate)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)
}

func TestDocumentMetadataAPIShapeHandleRetriesPublicationFailure(t *testing.T) {
	baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
	valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}
	valueB := documentMetadataAPIShapeTestValue{"B", "Description B"}
	publications := make([]documentMetadataValue, 0, 2)
	failNext := false
	coordinator := newDocumentMetadataCoordinator(
		baseline.private(),
		func(_ documentMetadataValue, next documentMetadataValue) error {
			if failNext {
				failNext = false
				return errors.New("forced publication failure")
			}
			publications = append(publications, next)
			return nil
		},
		nil,
	)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)

	value := valueA.public()
	var handle *DocumentMetadataOwner
	instance := testComponentInstance("RetryHandlePublication", func() Node {
		handle = UseDocumentMetadataOwner()
		UseOwnedDocumentMetadata(handle, value)
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(instance)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)

	value = valueB.public()
	failNext = true
	coordinator.beginUpdate()
	renderComponentInstance(instance)
	if recovered := captureDocumentMetadataAPIShapePanic(coordinator.commitUpdate); recovered == nil {
		t.Fatal("publication failure did not panic")
	}
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
	if handle.ID() != 1 || len(publications) != 1 {
		t.Fatalf("failed publication handle ID=%d publications=%#v", handle.ID(), publications)
	}

	coordinator.beginUpdate()
	renderComponentInstance(instance)
	coordinator.commitUpdate()
	assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueB, 1)
	if handle.ID() != 1 || !reflect.DeepEqual(publications, []documentMetadataValue{
		valueA.private(),
		valueB.private(),
	}) {
		t.Fatalf("retry handle ID=%d publications=%#v", handle.ID(), publications)
	}
}

func TestDocumentMetadataAPIShapeConditionalOwnershipRemount(t *testing.T) {
	baseline := documentMetadataAPIShapeTestValue{"Authored", "Baseline"}
	valueA := documentMetadataAPIShapeTestValue{"A", "Description A"}

	t.Run("hook", func(t *testing.T) {
		coordinator, _ := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
		host := testComponentInstance("ConditionalHookHost", func() Node {
			return Empty()
		}, nil)
		newOwner := func() *componentInstance {
			return testComponentInstanceWithParent(
				"ConditionalHookOwner",
				host,
				func() Node {
					UseDocumentMetadata(valueA.public())
					return Empty()
				},
			)
		}
		owner := newOwner()

		coordinator.beginUpdate()
		renderComponentInstance(host)
		renderComponentInstance(owner)
		coordinator.commitUpdate()
		firstID := coordinator.snapshot().owner.id
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)

		coordinator.beginUpdate()
		deactivateComponent(owner)
		renderComponentInstance(host)
		coordinator.commitUpdate()
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)

		coordinator.beginUpdate()
		renderComponentInstance(host)
		renderComponentInstance(newOwner())
		coordinator.commitUpdate()
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
		if coordinator.snapshot().owner.id <= firstID {
			t.Fatalf("hook remount owner ID = %d, want greater than %d", coordinator.snapshot().owner.id, firstID)
		}
	})

	t.Run("component", func(t *testing.T) {
		coordinator, _ := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
		host := testComponentInstance("ConditionalComponentHost", func() Node {
			return Empty()
		}, nil)
		newOwner := func() *componentInstance {
			return testComponentInstanceWithParent(
				"ConditionalMetadataComponent",
				host,
				func() Node {
					return DocumentMetadataComponent(DocumentMetadataComponentProps{
						Metadata: valueA.public(),
					})
				},
			)
		}
		owner := newOwner()

		coordinator.beginUpdate()
		renderComponentInstance(host)
		renderComponentInstance(owner)
		coordinator.commitUpdate()
		firstID := coordinator.snapshot().owner.id
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)

		coordinator.beginUpdate()
		deactivateComponent(owner)
		renderComponentInstance(host)
		coordinator.commitUpdate()
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)

		coordinator.beginUpdate()
		renderComponentInstance(host)
		renderComponentInstance(newOwner())
		coordinator.commitUpdate()
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)
		if coordinator.snapshot().owner.id <= firstID {
			t.Fatalf("component remount owner ID = %d, want greater than %d", coordinator.snapshot().owner.id, firstID)
		}
	})

	t.Run("handle", func(t *testing.T) {
		coordinator, publications := installDocumentMetadataAPIShapeTestCoordinator(t, baseline)
		var handle *DocumentMetadataOwner
		host := testComponentInstance("StableHandleHost", func() Node {
			current := UseDocumentMetadataOwner()
			if handle != nil && current != handle {
				panic("document metadata API shape test: stable handle changed")
			}
			handle = current
			return Empty()
		}, nil)
		newPublication := func() *componentInstance {
			return testComponentInstanceWithParent(
				"ConditionalHandlePublication",
				host,
				func() Node {
					UseOwnedDocumentMetadata(handle, valueA.public())
					return Empty()
				},
			)
		}
		publication := newPublication()

		coordinator.beginUpdate()
		renderComponentInstance(host)
		renderComponentInstance(publication)
		coordinator.commitUpdate()
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, valueA, 1)

		coordinator.beginUpdate()
		deactivateComponent(publication)
		coordinator.commitUpdate()
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)
		if handle.ActivePublications() != 0 || handle.ID() != 1 {
			t.Fatalf("released handle ID=%d publications=%d, want ID=1 publications=0", handle.ID(), handle.ActivePublications())
		}

		// The experiment intentionally preserves this released-handle limitation
		// instead of extending the candidate with owner renewal or resurrection.
		releasedHandle := handle
		beforeStatistics := coordinator.statistics
		beforePublications := len(*publications)
		beforeNextID := coordinator.nextID
		coordinator.beginUpdate()
		recovered := captureDocumentMetadataAPIShapePanic(func() {
			renderComponentInstance(host)
		})
		coordinator.discardUpdate()
		if recovered != "goframe: document metadata owner is already released" {
			t.Fatalf("stable handle reuse panic = %v, want released-owner diagnostic", recovered)
		}
		if handle != releasedHandle {
			t.Fatal("released stable handle object changed")
		}
		if handle.ActivePublications() != 0 || handle.ID() != 1 {
			t.Fatalf("rejected reuse handle ID=%d publications=%d, want ID=1 publications=0", handle.ID(), handle.ActivePublications())
		}
		assertDocumentMetadataAPIShapeSnapshot(t, coordinator, baseline, 0)
		if coordinator.nextID != beforeNextID ||
			!reflect.DeepEqual(coordinator.statistics, beforeStatistics) ||
			len(*publications) != beforePublications ||
			len(coordinator.owners) != 0 ||
			len(coordinator.pendingHandoffs) != 0 ||
			len(coordinator.pendingHandoffOrder) != 0 ||
			len(coordinator.pendingFinalizations) != 0 ||
			len(coordinator.pendingFinalizationOrder) != 0 {
			t.Fatalf(
				"rejected reuse mutated coordinator: nextID=%d statistics=%#v publications=%#v owners=%#v plans=%#v planOrder=%#v finalizations=%#v finalizationOrder=%#v",
				coordinator.nextID,
				coordinator.statistics,
				*publications,
				coordinator.owners,
				coordinator.pendingHandoffs,
				coordinator.pendingHandoffOrder,
				coordinator.pendingFinalizations,
				coordinator.pendingFinalizationOrder,
			)
		}
	})
}

type documentMetadataAPIShapeTestValue struct {
	title       string
	description string
}

func (value documentMetadataAPIShapeTestValue) public() DocumentMetadata {
	return DocumentMetadata{
		Title:       value.title,
		Description: value.description,
	}
}

func (value documentMetadataAPIShapeTestValue) private() documentMetadataValue {
	return documentMetadataValue{
		title:       value.title,
		description: value.description,
	}
}

func installDocumentMetadataAPIShapeTestCoordinator(
	t *testing.T,
	baseline documentMetadataAPIShapeTestValue,
) (*documentMetadataCoordinator, *[]documentMetadataValue) {
	t.Helper()
	publications := make([]documentMetadataValue, 0)
	coordinator := newDocumentMetadataCoordinator(
		baseline.private(),
		testDocumentMetadataPublisher(func(value documentMetadataValue) {
			publications = append(publications, value)
		}),
		nil,
	)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	return coordinator, &publications
}

func assertDocumentMetadataAPIShapeSnapshot(
	t *testing.T,
	coordinator *documentMetadataCoordinator,
	want documentMetadataAPIShapeTestValue,
	ownerCount int,
) {
	t.Helper()
	snapshot := coordinator.snapshot()
	if snapshot.metadata != want.private() || snapshot.ownerCount != ownerCount || snapshot.batchActive {
		t.Fatalf("snapshot = %#v, want metadata=%#v ownerCount=%d inactive batch", snapshot, want.private(), ownerCount)
	}
}

func useOwnedDocumentMetadataAPIShapeTestHelper(
	owner *DocumentMetadataOwner,
	metadata DocumentMetadata,
) {
	UseOwnedDocumentMetadata(owner, metadata)
}

func captureDocumentMetadataAPIShapePanic(operation func()) (recovered any) {
	defer func() {
		recovered = recover()
	}()
	operation()
	return nil
}
