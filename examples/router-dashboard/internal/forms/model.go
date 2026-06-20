package forms

import (
	"strings"

	data "github.com/graybuton/goframe/examples/router-dashboard/internal/data"
)

type IssueFormProps struct {
	Issue data.Issue
}

type FieldState struct {
	Value   string
	Error   string
	Touched bool
	Dirty   bool
}

type IssueFormState struct {
	Initial     data.Issue
	Title       FieldState
	Status      FieldState
	Description FieldState
	Submitted   bool
	Saved       bool
}

type IssueFormAction struct {
	Kind  issueFormActionKind
	Value string
}

type issueFormActionKind int

const (
	issueFormTitleChanged issueFormActionKind = iota + 1
	issueFormStatusChanged
	issueFormDescriptionChanged
	issueFormSubmit
	issueFormReset
)

func newIssueFormState(issue data.Issue) IssueFormState {
	state := IssueFormState{
		Initial: issue,
		Title: FieldState{
			Value: issue.Title,
		},
		Status: FieldState{
			Value: data.NormalizeStatus(issue.Status),
		},
		Description: FieldState{
			Value: issue.Description,
		},
	}
	state.validate()
	return state
}

func reduceIssueForm(state IssueFormState, action IssueFormAction) IssueFormState {
	switch action.Kind {
	case issueFormTitleChanged:
		state.Title.Value = action.Value
		state.Title.Touched = true
		state.Saved = false
	case issueFormStatusChanged:
		state.Status.Value = data.NormalizeStatus(action.Value)
		state.Status.Touched = true
		state.Saved = false
	case issueFormDescriptionChanged:
		state.Description.Value = action.Value
		state.Description.Touched = true
		state.Saved = false
	case issueFormSubmit:
		state.Submitted = true
		state.Title.Touched = true
		state.Status.Touched = true
		state.Description.Touched = true
	case issueFormReset:
		return newIssueFormState(state.Initial)
	}
	state.validate()
	if action.Kind == issueFormSubmit && state.Title.Error == "" {
		state.Saved = true
	}
	return state
}

func (state IssueFormState) ShowTitleError() bool {
	return state.Title.Error != "" && (state.Title.Touched || state.Submitted)
}

func (state IssueFormState) TitleInvalid() string {
	if state.ShowTitleError() {
		return "true"
	}
	return "false"
}

func (state IssueFormState) DirtyText() string {
	if state.Title.Dirty || state.Status.Dirty || state.Description.Dirty {
		return "Unsaved local changes"
	}
	return "No local changes"
}

func (state *IssueFormState) validate() {
	state.Title.Error = validateTitle(state.Title.Value)
	state.Title.Dirty = state.Title.Value != state.Initial.Title
	state.Status.Dirty = state.Status.Value != data.NormalizeStatus(state.Initial.Status)
	state.Description.Dirty = state.Description.Value != state.Initial.Description
}

func validateTitle(value string) string {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return "Title is required."
	case len(value) < 4:
		return "Title must be at least 4 characters."
	default:
		return ""
	}
}
