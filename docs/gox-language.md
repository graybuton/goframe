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

GOX also reports focused diagnostics for unclosed tags, invalid component
names, invalid component prop names, empty child/attribute expressions,
duplicate or valueless `Key` pseudo-props, spread props, and namespace tags.
Nested GOX markup inside callback return expressions is also reported against
the original `.gox` file rather than only the generated `.goframe/work` output.

Unsupported namespace tags remain unsupported. The diagnostic points users back
to ordinary Go imports and function calls for cross-package composition:

```text
examples/app.gox:8:10: namespace tags are not supported; use ordinary Go imports and function calls for cross-package composition: <ui.Header>
  <ui.Header />
```

Unsupported spread props remain unsupported. Pass explicit props instead:

```text
examples/app.gox:8:15: spread props are not supported; pass explicit props instead: {...props}
  <Button {...props} />
```

Go type errors, such as a missing props struct or invalid field type, are
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
- component namespaces or dotted tags;
- style objects;
- lifecycle/effect-specific GOX syntax; use Go calls such as `gf.UseEffect`;
- advanced reconciliation controls;
- async components;
- routing;
- SSR or hydration;
- compile-time validation that a component function or props struct exists.

Use normal Go before the return expression to prepare values and choose what a
component receives.
