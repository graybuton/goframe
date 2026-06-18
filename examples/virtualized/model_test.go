package main

import "testing"

func TestMakeDemoItemsIsDeterministic(t *testing.T) {
	items := makeDemoItems(3)
	if len(items) != 3 {
		t.Fatalf("len = %d, want 3", len(items))
	}
	if items[0].ID != 1 || items[0].Name != "Item 0001" || items[1].Group != "Beta" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestReduceDemoItemsToggleDoesNotMutateInput(t *testing.T) {
	items := makeDemoItems(4)
	before := items[1].Enabled
	next := reduceDemoItems(items, DemoAction{Kind: DemoActionToggle, ID: 2})
	if items[1].Enabled != before {
		t.Fatalf("input mutated: got %v, want %v", items[1].Enabled, before)
	}
	if next[1].Enabled == before {
		t.Fatalf("toggle did not change item 2")
	}
}

func TestFindDemoItem(t *testing.T) {
	items := makeDemoItems(4)
	item, ok := findDemoItem(items, 3)
	if !ok || item.ID != 3 {
		t.Fatalf("find item = %#v, %v", item, ok)
	}
	if _, ok := findDemoItem(items, 99); ok {
		t.Fatal("unexpected match for missing item")
	}
}
