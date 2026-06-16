package goframe

import "testing"

func TestPruneDirtyComponents(t *testing.T) {
	root := dirtyTestInstance("Root", nil)
	parent := dirtyTestInstance("Parent", root)
	child := dirtyTestInstance("Child", parent)
	grandchild := dirtyTestInstance("GrandChild", child)
	sibling := dirtyTestInstance("Sibling", root)
	keyedA := dirtyTestInstance("TodoItem", parent)
	keyedA.key = "a"
	keyedB := dirtyTestInstance("TodoItem", parent)
	keyedB.key = "b"
	inactive := dirtyTestInstance("Inactive", parent)
	deactivateComponent(inactive)

	tests := []struct {
		name  string
		dirty []*componentInstance
		want  []*componentInstance
	}{
		{
			name:  "one dirty component",
			dirty: []*componentInstance{child},
			want:  []*componentInstance{child},
		},
		{
			name:  "parent prunes child",
			dirty: []*componentInstance{parent, child},
			want:  []*componentInstance{parent},
		},
		{
			name:  "parent prunes multiple descendants",
			dirty: []*componentInstance{parent, child, grandchild},
			want:  []*componentInstance{parent},
		},
		{
			name:  "siblings remain scheduled",
			dirty: []*componentInstance{child, sibling},
			want:  []*componentInstance{child, sibling},
		},
		{
			name:  "ancestor and sibling",
			dirty: []*componentInstance{parent, child, sibling},
			want:  []*componentInstance{parent, sibling},
		},
		{
			name:  "reverse order child before ancestor",
			dirty: []*componentInstance{child, parent},
			want:  []*componentInstance{parent},
		},
		{
			name:  "unmounted dirty component skipped",
			dirty: []*componentInstance{inactive, child},
			want:  []*componentInstance{child},
		},
		{
			name:  "duplicate enqueue kept once",
			dirty: []*componentInstance{child, child, sibling},
			want:  []*componentInstance{child, sibling},
		},
		{
			name:  "keyed reorder siblings not pruned",
			dirty: []*componentInstance{keyedB, keyedA},
			want:  []*componentInstance{keyedB, keyedA},
		},
		{
			name:  "root prunes child",
			dirty: []*componentInstance{child, root},
			want:  []*componentInstance{root},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := pruneDirtyComponents(test.dirty)
			assertInstances(t, got, test.want)
		})
	}
}

func dirtyTestInstance(name string, parent *componentInstance) *componentInstance {
	node := Component(name, struct{}{}, func(struct{}) Node { return Empty() }).(ComponentNode)
	instance := newComponentInstance(node, "", parent, nil)
	instance.dirty = true
	return instance
}

func assertInstances(t *testing.T, got, want []*componentInstance) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("instances = %s, want %s", instanceNames(got), instanceNames(want))
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("instances = %s, want %s", instanceNames(got), instanceNames(want))
		}
	}
}

func instanceNames(instances []*componentInstance) []string {
	names := make([]string, len(instances))
	for index, instance := range instances {
		if instance == nil {
			names[index] = "<nil>"
			continue
		}
		names[index] = instance.name
		if instance.key != "" {
			names[index] += ":" + instance.key
		}
	}
	return names
}
