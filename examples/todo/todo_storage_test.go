package main

import "testing"

func TestEncodeDecodeTodos(t *testing.T) {
	todos := []Todo{
		{ID: 1, Text: "plain"},
		{ID: 2, Text: "pipe|slash\\line\nnext", Done: true},
	}

	decoded := decodeTodos(encodeTodos(todos))
	if len(decoded) != len(todos) {
		t.Fatalf("decoded len = %d, want %d", len(decoded), len(todos))
	}
	for index := range todos {
		if decoded[index] != todos[index] {
			t.Fatalf("decoded[%d] = %#v, want %#v", index, decoded[index], todos[index])
		}
	}
}

func TestNextTodoID(t *testing.T) {
	if got := nextTodoID([]Todo{{ID: 4}, {ID: 2}}); got != 5 {
		t.Fatalf("next id = %d, want 5", got)
	}
	if got := nextTodoID(nil); got != 1 {
		t.Fatalf("empty next id = %d, want 1", got)
	}
}
