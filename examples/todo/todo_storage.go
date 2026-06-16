package main

const todoStorageKey = "goframe.todo.items"

var todoSkipNextPersist bool

func encodeTodos(todos []Todo) string {
	if len(todos) == 0 {
		return ""
	}
	out := make([]byte, 0, len(todos)*16)
	for index, todo := range todos {
		if index > 0 {
			out = append(out, '\n')
		}
		out = appendPositiveInt(out, todo.ID)
		out = append(out, '|')
		if todo.Done {
			out = append(out, '1')
		} else {
			out = append(out, '0')
		}
		out = append(out, '|')
		out = appendEscapedTodoText(out, todo.Text)
	}
	return string(out)
}

func decodeTodos(raw string) []Todo {
	if raw == "" {
		return nil
	}
	todos := make([]Todo, 0, 4)
	start := 0
	for start <= len(raw) {
		end := start
		for end < len(raw) && raw[end] != '\n' {
			end++
		}
		todo, ok := decodeTodoLine(raw[start:end])
		if ok {
			todos = append(todos, todo)
		}
		if end == len(raw) {
			break
		}
		start = end + 1
	}
	return todos
}

func decodeTodoLine(line string) (Todo, bool) {
	first := indexByte(line, '|')
	if first <= 0 {
		return Todo{}, false
	}
	second := first + 1 + indexByte(line[first+1:], '|')
	if second <= first {
		return Todo{}, false
	}
	id, ok := parsePositiveInt(line[:first])
	if !ok {
		return Todo{}, false
	}
	return Todo{
		ID:   id,
		Done: line[first+1:second] == "1",
		Text: unescapeTodoText(line[second+1:]),
	}, true
}

func parsePositiveInt(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	result := 0
	for index := 0; index < len(value); index++ {
		digit := value[index]
		if digit < '0' || digit > '9' {
			return 0, false
		}
		result = result*10 + int(digit-'0')
	}
	return result, true
}

func appendPositiveInt(out []byte, value int) []byte {
	if value <= 0 {
		return append(out, '0')
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value = value / 10
	}
	return append(out, digits[index:]...)
}

func indexByte(value string, target byte) int {
	for index := 0; index < len(value); index++ {
		if value[index] == target {
			return index
		}
	}
	return -1
}

func hasByte(value string, target byte) bool {
	return indexByte(value, target) >= 0
}

func appendEscapedTodoText(out []byte, text string) []byte {
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		case '|':
			out = append(out, '\\', 'p')
		default:
			out = append(out, text[index])
		}
	}
	return out
}

func unescapeTodoText(text string) string {
	if !hasByte(text, '\\') {
		return text
	}
	out := make([]byte, 0, len(text))
	escaped := false
	for index := 0; index < len(text); index++ {
		value := text[index]
		if escaped {
			switch value {
			case 'n':
				out = append(out, '\n')
			case 'p':
				out = append(out, '|')
			default:
				out = append(out, value)
			}
			escaped = false
			continue
		}
		if value == '\\' {
			escaped = true
			continue
		}
		out = append(out, value)
	}
	if escaped {
		out = append(out, '\\')
	}
	return string(out)
}

func nextTodoID(todos []Todo) int {
	next := 1
	for _, todo := range todos {
		if todo.ID >= next {
			next = todo.ID + 1
		}
	}
	return next
}
