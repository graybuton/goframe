# Forms and Validation

## Purpose

GoFrame does not include a form framework in MVP 25. This document describes
the recommended patterns for controlled inputs, submit handling, validation,
touched/dirty state, and reset behavior using the existing runtime primitives.

The goal is to give applications a clear starting point without adding schema
validation, async submit, server actions, or a form registry to the runtime.

## Current Primitive Model

Forms are ordinary components:

- keep form state in `gf.UseState` or `gf.UseReducer`;
- use `gf.InputEvent.Value()` from `OnInput` or `OnChange`;
- call `gf.Event.PreventDefault()` from `OnSubmit`;
- render validation messages as normal conditional UI;
- run validation in ordinary Go functions.

## Controlled Inputs

A controlled input receives its value from component state and writes changes
back through an event handler:

```gox
<input
    Value={state.Title}
    OnInput={func(event gf.InputEvent) {
        dispatch(FormAction{
            Kind: FormActionTitleChanged,
            Value: event.Value(),
        })
    }}
/>
```

The runtime keeps one browser listener per event name and updates the current
Go callback during patching. Reducer dispatch is the preferred pattern when
callbacks may be retained by memoized children, because dispatch reads the
latest state slot at event time.

## Submit Handling

Use `OnSubmit` on the `<form>` and prevent the browser's default navigation:

```gox
<form OnSubmit={func(event gf.Event) {
    event.PreventDefault()
    dispatch(FormAction{Kind: FormActionSubmit})
}}>
    ...
</form>
```

Submit remains synchronous in MVP 25. There is no server action, async
resource, or route loader integration.

## Field State

A practical field model is application-owned:

```go
type FieldState struct {
    Value   string
    Error   string
    Touched bool
    Dirty   bool
}
```

GoFrame does not define this type in the runtime. Keeping it in the app avoids
turning the runtime into a validation library and lets each app choose the
shape it needs.

## Validation State

Validation is plain Go:

```go
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
```

Run validation on input, blur, submit, or any combination your UX needs. The
reference example validates on input after a field has been touched and always
validates on submit.

## Touched / Dirty Patterns

Common flags:

- `Touched`: the user interacted with the field;
- `Dirty`: the current value differs from the initial value;
- `Submitted`: the form has attempted submit at least once.

A typical reducer updates these flags alongside the value:

```go
case FormActionTitleChanged:
    state.Title.Value = action.Value
    state.Title.Touched = true
    state.Title.Dirty = state.Title.Value != state.InitialTitle
```

## Error Display

Render errors only when they should be visible:

```gox
{(field.Error != "" && (field.Touched || state.Submitted)) && (
    <p class="field-error">{field.Error}</p>
)}
```

Use ordinary attributes such as `aria-invalid` and `aria-describedby` where
appropriate. GoFrame does not add an accessibility abstraction for forms yet.

## Form Reset

Reset can be a normal reducer action:

```gox
<button Type="button" OnClick={func() {
    dispatch(FormAction{Kind: FormActionReset})
}}>
    Reset
</button>
```

The reset action should restore initial values, clear errors, and clear
touched/dirty/submitted flags according to the app's desired UX.

## Browser Event Semantics

Useful event facades:

- `gf.Event.PreventDefault()`
- `gf.Event.StopPropagation()`
- `gf.InputEvent.Value()`

`InputEvent.Value()` returns an empty string when no target value is available.
It keeps `syscall/js` out of app-facing form code.

## Recommended Patterns

For small forms:

- use `UseState` for one or two fields;
- validate in small local functions;
- keep submit state local to the form component.

For larger forms:

- use `UseReducer`;
- define explicit action kinds;
- keep field value/error/touched/dirty together;
- make validation deterministic and synchronous;
- keep navigation after successful submit explicit with `gf.Navigate`.

When a stateful form lives in another Go package, prefer a package-qualified
component tag:

```gox
<forms.IssueForm Key={issue.ID} Issue={issue} />
```

That keeps component composition declarative and still gives generated code a
runtime component boundary. Calling a stateful render function directly from
another package is just an ordinary Go function call and does not create its
own component boundary.

## What GoFrame Does Not Provide Yet

GoFrame does not provide:

- schema validation;
- async validation;
- server submit actions;
- automatic field registration;
- form context;
- route loaders/resources;
- query-state binding;
- browser-native constraint validation wrappers;
- file upload helpers.

## Future Work

Future work may add small helpers if repeated app patterns prove stable, but
that should be separate from MVP 25 and measured against bundle size and API
surface growth.
