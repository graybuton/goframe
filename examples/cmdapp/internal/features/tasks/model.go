package tasks

type Task struct {
	ID       string
	Title    string
	Priority string
}

type TaskListProps struct {
	Items []Task
}

type TaskRowProps struct {
	Task Task
}

func DemoTasks() []Task {
	return []Task{
		{ID: "CE-1", Title: "Build child entry package", Priority: "high"},
		{ID: "CE-2", Title: "Generate internal GOX packages", Priority: "medium"},
		{ID: "CE-3", Title: "Keep source tree clean", Priority: "low"},
	}
}
