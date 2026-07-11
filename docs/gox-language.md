# GOX language

GOX is a small JSX-like syntax embedded in otherwise normal Go source. `goxc
generate` converts markup roots into calls to the `goframe` runtime and
explicit component boundaries.

GOX deliberately keeps Go as the language for state, functions, types, and
control flow. It is not a separate scripting language.

## HTML elements

Tags beginning with a lowercase letter are rendered as HTML elements:

```gox
<main class="page">
    <input type="text" />
    <p>Hello</p>
</main>
```

Generated shape:

```go
gf.El("main", gf.Props{
    "class": "page",
},
    gf.El("input", gf.Props{"type": "text"}),
    gf.El("p", nil, gf.Text("Hello")),
)
```

HTML attributes become keys in `gf.Props`. Event props such as `onClick` are
handled by the browser runtime.

## Function components

Tags beginning with an uppercase letter create a component boundary around a
Go component function:

```gox
<Button Label="Increment" OnClick={increment} />
```

GOX uses the convention `<ComponentName>Props`:

```go
type ButtonProps struct {
    Label   string
    OnClick func()
}

func Button(props ButtonProps) gf.Node {
    return (
        <button onClick={props.OnClick}>{props.Label}</button>
    )
}
```

Generated component node:

```go
var _goxComponent_app_Button = gf.NewComponentType("main.Button", "Button")

gf.ComponentT(_goxComponent_app_Button, ButtonProps{
    Label:   "Increment",
    OnClick: increment,
}, Button)
```

Every capitalized tag uses the props struct convention, including components
without attributes:

```gox
<Status />
```

```go
gf.ComponentT(_goxComponent_app_Status, StatusProps{}, Status)
```

Component prop names must be valid Go field names. Go's type checker reports
unknown fields and incompatible prop values after generation.

The boundary gives the runtime a component instance, scoped state slots, and a
mounted subtree that can be updated independently of ancestors and siblings.
Generated GOX code uses `gf.NewComponentType`/`gf.ComponentT` so the runtime has
an explicit identity token separate from the short debug name. Calling
`Button(ButtonProps{...})` directly is still valid Go, but it is an ordinary
function call without separate component identity.

When `goxc` knows the package import path, generated component ids use that
path plus the component name, for example
`github.com/graybuton/goframe/examples/multipackage/internal/ui.Header`.
Lower-level generation helpers can still fall back to package-name ids such as
`main.Header`.

## Package-qualified component tags

GOX supports Go-like package-qualified component tags for cross-package
composition:

```gox
import ui "github.com/example/app/internal/ui"

func App() gf.Node {
    return (
        <ui.Shell Title="Dashboard">
            <p>Hello</p>
        </ui.Shell>
    )
}
```

This is not an XML namespace system. The supported form is exactly:

```text
packageAlias.Component
```

Rules:

- `packageAlias` must be a Go import alias in the current `.gox` file;
- implicit aliases from import path bases are supported;
- blank and dot imports cannot be used for qualified component tags;
- `Component` must be an exported Go identifier;
- props use the `packageAlias.ComponentProps` convention;
- nested children map to `Children []gf.Node`;
- `Key` remains a pseudo-prop and is not passed into the props struct.

Generated shape:

```go
var _goxComponent_app_ui_Shell = gf.NewComponentType(
    "github.com/example/app/internal/ui.Shell",
    "ui.Shell",
)

gf.ComponentT(_goxComponent_app_ui_Shell, ui.ShellProps{
    Title: "Dashboard",
    Children: []gf.Node{
        gf.El("p", nil, gf.Text("Hello")),
    },
}, ui.Shell)
```

Component identity uses the resolved import path plus component name, not the
local alias. The debug name uses `alias.Component`, which keeps browser smoke
and debug counters readable when several packages define `Header` or `Shell`.

Useful examples:

```gox
<layout.Shell>
    {gf.RouterView(router)}
</layout.Shell>

<gf.RouterLink To="/issues" Class="nav-link">
    Issues
</gf.RouterLink>

<filters.FilterControls
    Query={props.Query}
    Status={props.Status}
    ResultCount={len(props.Items)}
    TotalCount={props.TotalCount}
/>
```

Unsupported forms:

```gox
<ui:Shell />       // XML-style namespace syntax
<foo.bar.Shell />  // nested selector chain
<ui.shell />       // selected component is not exported
<.Shell />         // dot imports are not supported for tags
```

Use ordinary Go expressions for values that are not component composition, such
as `gf.RouterView(router)`, `gf.Map(...)`, or helper formatting functions.

## Children

A component receives nested markup through a `Children []gf.Node` field:

```go
type CardProps struct {
    Title    string
    Children []gf.Node
}
```

```gox
<Card Title="Stats">
    <p>Counter: {count}</p>
</Card>
```

Generated shape:

```go
gf.ComponentT(_goxComponent_app_Card, CardProps{
    Title: "Stats",
    Children: []gf.Node{
        gf.El("p", nil,
            gf.Text("Counter: "),
            gf.Child(count),
        ),
    },
}, Card)
```

Render component children inside another component with an expression:

```gox
<div class="card-body">
    {props.Children}
</div>
```

`gf.Child` accepts a primitive value, one `gf.Node`, or `[]gf.Node`.
`[]gf.Node` becomes a fragment, so no wrapper element is added.

This makes Go-native list helpers usable directly in children:

```gox
<ul>
    {gf.Map(items, func(item Item) gf.Node {
        return <ItemRow Key={item.ID} Item={item} />
    })}
</ul>
```

## Fragments

Fragments group sibling nodes without creating an HTML wrapper:

```gox
<>
    <h1>Hello</h1>
    <p>World</p>
</>
```

Generated shape:

```go
gf.Fragment(
    gf.El("h1", nil, gf.Text("Hello")),
    gf.El("p", nil, gf.Text("World")),
)
```

The browser renderer uses a DOM `DocumentFragment`.

## Expressions

Child expressions use braces:

```gox
<p>Current value: {count}</p>
```

They generate `gf.Child(expression)`. Supported runtime values are:

- `gf.Node`;
- `[]gf.Node`;
- strings, numbers, and booleans supported by `gf.ToString`.

Attribute and component prop expressions are inserted as normal Go
expressions:

```gox
<Button OnClick={func() {
    setCount(count + 1)
}} />
```

GOX expressions may contain nested GOX markup inside Go callbacks when that
markup appears in a return expression:

```gox
{gf.Map(items, func(item Item) gf.Node {
    if item.Hidden {
        return <></>
    }
    return <ItemRow Key={item.ID} Item={item} />
})}
```

The compiler rewrites the nested `return <ItemRow />` into normal generated Go.

## Conditional rendering

GOX intentionally does not define template-block `if` syntax. Instead it adds
two JSX-like render expressions inside child expression braces.

`condition && <Node />` renders a node only when the condition is true:

```gox
<main>
    {ready && <ReadyView />}
    {(len(items) > 0) && (
        <p>{len(items)} item(s)</p>
    )}
</main>
```

Generated shape:

```go
gf.If(ready, gf.ComponentT(_goxComponent_app_ReadyView, ReadyViewProps{}, ReadyView))
```

Ordinary Go boolean expressions remain ordinary child expressions when the
right-hand side is not GOX markup:

```gox
{ready && active}
```

```go
gf.Child(ready && active)
```

`condition ? nodeA : nodeB` selects one of two GOX node branches:

```gox
{len(items) == 0 ? (
    <EmptyState />
) : (
    <ItemList Items={items} />
)}
```

Generated shape:

```go
gf.IfElse(
    len(items) == 0,
    gf.ComponentT(_goxComponent_app_EmptyState, EmptyStateProps{}, EmptyState),
    gf.ComponentT(_goxComponent_app_ItemList, ItemListProps{Items: items}, ItemList),
)
```

Both ternary branches must be GOX node expressions, components, elements,
fragments, or parenthesized GOX node expressions. This is GOX syntax, not a
general Go ternary operator.

## List rendering

Lists stay Go-native through helper functions:

```gox
{gf.Map(items, func(item Item) gf.Node {
    return <ItemRow Key={item.ID} Item={item} />
})}
```

`gf.MapIndexed` also provides the item index:

```gox
{gf.MapIndexed(items, func(index int, item Item) gf.Node {
    return <li Key={index}>{item.Label}</li>
})}
```

`gf.For` and `gf.ForIndexed` still exist as deprecated runtime aliases for
generated-code-like Go. New examples and docs should prefer `gf.Map` and
`gf.MapIndexed`.

## Keys

`Key` is a GOX pseudo-prop:

```gox
<TodoItem Key={todo.ID} Todo={todo} />
<li Key="static-row">{label}</li>
```

It is not passed into component props and is not emitted as an HTML attribute.
Generated code wraps the node:

```go
gf.Key(gf.ToString(todo.ID),
    gf.ComponentT(_goxComponent_app_TodoItem, TodoItemProps{Todo: todo}, TodoItem),
)
```

Keys provide stable sibling identity to the minimal DOM patch and component
layers. A key around a generated component boundary preserves that component instance,
scoped state, and compatible mounted DOM range across removal and reorder.

## Events and controlled inputs

Common DOM props and event names support lowercase and exported-style forms:

```gox
<input
    Type="text"
    Value={text}
    Placeholder="New task"
    OnInput={func(event gf.InputEvent) {
        setText(event.Value())
    }}
/>
```

Handlers may be `func()`, `func(gf.Event)`, or `func(gf.InputEvent)`.
`gf.Event` exposes `PreventDefault` and `StopPropagation`.

Controlled inputs retain their DOM node while their value is patched. A stable
`ID` remains useful as a focus-restoration fallback when a node must be
replaced.

## Self-closing tags

Both HTML elements and components may be self-closing:

```gox
<input />
<br />
<img src="logo.png" />
<Button Label="OK" />
```

## Diagnostics

Parser errors include filename, global line, column, description, and the
relevant source line:

```text
examples/app.gox:12:18: expected closing tag </div>, got </main>
  <div>Broken</main>
```

Run a read-only source check for one authored file or a directory tree:

```bash
goxc check ./examples/counter
goxc check ./examples/counter --format=json
```

Directory checks recursively validate authored `.gox` files in nested packages
using the same discovery and package-identity rules as generation. Files are
processed in deterministic lexical order. Symlinked source paths are rejected,
and the existing top-level skipped directories such as `.goframe`, `build`,
`dist`, `node_modules`, and `.git` are not scanned.

`goxc check` calls the GOX generator in memory and discards successful output.
It does not create `.goframe`, generated `.gox.go` files, build/package files,
or a generated workspace. A completed check exits `0` when no diagnostics are
found and `1` when source diagnostics exist. Argument, path, discovery, safety,
and source-read failures also exit `1` through the normal `goxc` operational
error path.

Text is the default format. Source diagnostics and the failed summary go to
stderr; a successful summary goes to stdout. JSON mode writes one compact JSON
document and a trailing newline to stdout. Completed JSON diagnostic reports do
not add human-readable output to stderr. GOX markup remains readable because
the encoder does not apply JSON HTML escaping.

The current machine-readable transport uses schema version 1:

```json
{"schemaVersion":1,"ok":false,"filesChecked":1,"diagnostics":[{"file":"/absolute/path/app.gox","line":4,"column":15,"severity":"error","message":"gox: empty child expression","source":"<main>{}</main>"}]}
```

The schema has these contracts:

- `schemaVersion` is the integer `1`; consumers should reject unsupported
  versions, and incompatible field or semantic changes require an increment;
- `ok` is true only when `diagnostics` is empty;
- `filesChecked` includes every processed `.gox` file, including files with a
  diagnostic;
- `diagnostics` is always an array and is ordered by file, line, column, then
  message;
- `file` is a cleaned absolute native filesystem path;
- `line` is one-based when available and `0` when unavailable;
- `column` is a one-based UTF-8 byte column when available and `0` when
  unavailable; editor consumers must translate it into their editor's position
  encoding;
- `severity` is currently always `"error"`;
- `source` is the relevant authored source line when available and an empty
  string otherwise.

The official lightweight VS Code extension consumes schema version 1 for
CLI-backed inline source diagnostics. This remains process-based editor
tooling, not an LSP. It checks saved authored source and does not diagnose
unsaved buffer content or run Go/TinyGo semantic type checking. Exact
diagnostic wording remains experimental. The existing generator currently
returns at most one compiler diagnostic per file, but a directory check
continues through all later files instead of stopping at the first failing
file. Operational failures do not produce a completed or partial JSON report.

GOX also reports focused diagnostics for unclosed tags, invalid component
names, invalid component prop names, empty child/attribute expressions,
duplicate or valueless `Key` pseudo-props, spread props, XML-style namespace
syntax, unknown package aliases, lowercase qualified component selectors, and
nested selector chains. Nested GOX markup inside callback return expressions is
also reported against the original `.gox` file rather than only the generated
`.goframe/work` output.

Package-qualified tags require an import alias. Unknown aliases get a focused
diagnostic:

```text
examples/app.gox:8:10: gox: unknown package alias "ui" in qualified component <ui.Header>; import the package or use a local component tag
  <ui.Header />
```

XML-style namespace tags remain unsupported:

```text
examples/app.gox:8:10: namespace tags with ':' are not supported; use package-qualified component tags like <ui.Header>
  <ui:Header />
```

Unsupported spread props remain unsupported. Pass explicit props instead:

```text
examples/app.gox:8:15: spread props are not supported; pass explicit props instead: {...props}
  <Button {...props} />
```

`goxc check` validates GOX parsing and code generation only. It does not run Go
or TinyGo type checking, scan remote modules, or process external dependency
`.gox` files outside the requested authored tree. Go type errors, such as an
unknown component prop, a missing props struct, or an invalid field type, are
reported by the selected Go or TinyGo compiler after generation. Those errors
may still mention the hidden `.goframe/work/<profile>` workspace because they
come from the Go toolchain, but `goxc` generation failures prefer the original
`.gox` source path.

## Current limitations

GOX does not currently support:

- dedicated template-block loop or `if` statements inside markup; use GOX
  render expressions and Go callbacks with `gf.Map`;
- arbitrary JavaScript-like expressions, arrow functions, or `items.map(...)`;
- spread props;
- XML-style namespace tags with `:`;
- nested selector chains beyond `packageAlias.Component`;
- style objects;
- lifecycle/effect-specific GOX syntax; use Go calls such as `gf.UseEffect`;
- routing-specific GOX syntax; use Go route declarations such as
  `gf.NewHashRouter`;
- advanced reconciliation controls;
- async components;
- SSR or hydration;
- compile-time validation that a component function or props struct exists.

Use normal Go before the return expression to prepare values and choose what a
component receives.
