package main

type Preferences struct {
	Density string
	Accent  string
	Count   int
}

const (
	DensityComfortable = "comfortable"
	DensityCompact     = "compact"

	AccentBlue   = "blue"
	AccentGreen  = "green"
	AccentPurple = "purple"
)

type PreferenceActionKind int

const (
	PreferenceActionDensity PreferenceActionKind = iota
	PreferenceActionAccent
	PreferenceActionIncrement
	PreferenceActionReset
)

type PreferenceAction struct {
	Kind  PreferenceActionKind
	Value string
}

func defaultPreferences() Preferences {
	return Preferences{
		Density: DensityComfortable,
		Accent:  AccentBlue,
		Count:   0,
	}
}

func reducePreferences(state Preferences, action PreferenceAction) Preferences {
	switch action.Kind {
	case PreferenceActionDensity:
		state.Density = action.Value
	case PreferenceActionAccent:
		state.Accent = action.Value
	case PreferenceActionIncrement:
		state.Count++
	case PreferenceActionReset:
		return defaultPreferences()
	}
	return state
}
