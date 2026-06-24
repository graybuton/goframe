package main

const (
	resourceOpenKey     = "assets/data/issues-open.txt"
	resourceAllKey      = "assets/data/issues-all.txt"
	resourceMissingKey  = "assets/data/missing.txt"
	resourceSlowOpenKey = "slow:assets/data/issues-open.txt"
)

type resourceAppState struct {
	Key       string
	ShowPanel bool
}

type resourceAppAction struct {
	Kind resourceAppActionKind
	Key  string
}

type resourceAppActionKind int

const (
	resourceAppSetKey resourceAppActionKind = iota + 1
	resourceAppTogglePanel
)

func initialResourceAppState() resourceAppState {
	return resourceAppState{
		Key:       resourceSlowOpenKey,
		ShowPanel: true,
	}
}

func reduceResourceApp(state resourceAppState, action resourceAppAction) resourceAppState {
	switch action.Kind {
	case resourceAppSetKey:
		state.Key = action.Key
		if !state.ShowPanel {
			state.ShowPanel = true
		}
	case resourceAppTogglePanel:
		state.ShowPanel = !state.ShowPanel
	}
	return state
}
