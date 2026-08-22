//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

const (
	fixtureLargeLength = 200
	fixtureShortLength = 2
	fixtureHeight      = 192
	fixtureItemHeight  = 48
	fixtureOverscan    = 2

	fixtureExpandedHeight    = 384
	fixtureExpandedOverscan  = 20
	fixtureChangedItemHeight = 40
)

type fixtureItem struct {
	ID int
}

type fixtureItemProps struct {
	Kind     string
	Item     fixtureItem
	Style    string
	Selected bool
	Toggled  bool
	OnSelect func(int)
	OnToggle func(int)
}

func main() {
	installFixtureAudit()
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}

func App() gf.Node {
	length, setLength := gf.UseState(fixtureLargeLength)
	height, setHeight := gf.UseState(fixtureHeight)
	itemHeight, setItemHeight := gf.UseState(fixtureItemHeight)
	overscan, setOverscan := gf.UseState(fixtureOverscan)
	listSelected, setListSelected := gf.UseState(-1)
	listToggled, setListToggled := gf.UseState(-1)
	tableSelected, setTableSelected := gf.UseState(-1)
	tableToggled, setTableToggled := gf.UseState(-1)

	fixtureIncrementScalar("appRenders")
	gf.UseEffect(func() gf.Cleanup {
		fixtureIncrementScalar("appMounts")
		return func() {
			fixtureIncrementScalar("appCleanups")
		}
	})

	items := fixtureItems(length)

	selectList := func(id int) {
		fixtureRecordInteraction("list", "select", id)
		setListSelected(id)
	}
	toggleList := func(id int) {
		fixtureRecordInteraction("list", "toggle", id)
		setListToggled(id)
	}
	selectTable := func(id int) {
		fixtureRecordInteraction("table", "select", id)
		setTableSelected(id)
	}
	toggleTable := func(id int) {
		fixtureRecordInteraction("table", "toggle", id)
		setTableToggled(id)
	}

	return gf.El("main", gf.Props{
		"data-testid":      "virtual-range-fixture",
		"data-length":      length,
		"data-height":      height,
		"data-item-height": itemHeight,
		"data-overscan":    overscan,
	},
		gf.El("h1", gf.Props{}, gf.Text("Virtual range restoration")),
		gf.El("div", gf.Props{"data-testid": "controls"},
			fixtureButton("control-large", "Large", func() { setLength(fixtureLargeLength) }),
			fixtureButton("control-short", "Short", func() { setLength(fixtureShortLength) }),
			fixtureButton("control-empty", "Empty", func() { setLength(0) }),
			fixtureButton("control-height-expand", "Expand height", func() { setHeight(fixtureExpandedHeight) }),
			fixtureButton("control-height-reset", "Reset height", func() { setHeight(fixtureHeight) }),
			fixtureButton("control-overscan-expand", "Expand overscan", func() { setOverscan(fixtureExpandedOverscan) }),
			fixtureButton("control-overscan-reset", "Reset overscan", func() { setOverscan(fixtureOverscan) }),
			fixtureButton("control-item-height-change", "Change item height", func() { setItemHeight(fixtureChangedItemHeight) }),
			fixtureButton("control-item-height-reset", "Reset item height", func() { setItemHeight(fixtureItemHeight) }),
			fixtureButton("control-reset", "Reset", func() {
				setLength(fixtureLargeLength)
				setHeight(fixtureHeight)
				setItemHeight(fixtureItemHeight)
				setOverscan(fixtureOverscan)
				setListSelected(-1)
				setListToggled(-1)
				setTableSelected(-1)
				setTableToggled(-1)
			}),
		),
		gf.El("p", gf.Props{"data-testid": "list-selection"}, gf.Text("list-selected:"+gf.ToString(listSelected))),
		gf.El("p", gf.Props{"data-testid": "list-toggle"}, gf.Text("list-toggled:"+gf.ToString(listToggled))),
		gf.El("p", gf.Props{"data-testid": "table-selection"}, gf.Text("table-selected:"+gf.ToString(tableSelected))),
		gf.El("p", gf.Props{"data-testid": "table-toggle"}, gf.Text("table-toggled:"+gf.ToString(tableToggled))),
		gf.VirtualList(gf.VirtualListProps[fixtureItem]{
			Items:      items,
			Height:     height,
			ItemHeight: itemHeight,
			Overscan:   overscan,
			Key: func(item fixtureItem, _ int) string {
				return gf.ToString(item.ID)
			},
			RenderItem: func(item gf.VirtualItem[fixtureItem]) gf.Node {
				return gf.Component("VirtualRangeFixtureListItem", fixtureItemProps{
					Kind:     "list",
					Item:     item.Item,
					Selected: item.Item.ID == listSelected,
					Toggled:  item.Item.ID == listToggled,
					OnSelect: selectList,
					OnToggle: toggleList,
				}, renderFixtureListItem)
			},
			TestID: "fixture-virtual-list",
		}),
		gf.VirtualTable(gf.VirtualTableProps[fixtureItem]{
			Items:       items,
			Height:      height,
			RowHeight:   itemHeight,
			Overscan:    overscan,
			ColumnCount: 2,
			Key: func(item fixtureItem, _ int) string {
				return gf.ToString(item.ID)
			},
			RenderRow: func(row gf.VirtualRow[fixtureItem]) gf.Node {
				return gf.Component("VirtualRangeFixtureTableRow", fixtureItemProps{
					Kind:     "table",
					Item:     row.Item,
					Style:    row.RowStyle,
					Selected: row.Item.ID == tableSelected,
					Toggled:  row.Item.ID == tableToggled,
					OnSelect: selectTable,
					OnToggle: toggleTable,
				}, renderFixtureTableRow)
			},
			Empty: func() gf.Node {
				return gf.El("span", gf.Props{"data-testid": "table-empty"}, gf.Text("empty"))
			},
			TestID: "fixture-virtual-table",
		}),
	)
}

func renderFixtureListItem(props fixtureItemProps) gf.Node {
	useFixtureItemLifetime(props.Kind, props.Item.ID)
	className := "fixture-list-item"
	if props.Selected {
		className += " selected"
	}
	if props.Toggled {
		className += " toggled"
	}
	return gf.El("div", gf.Props{
		"class":       className,
		"data-testid": "fixture-list-item-" + gf.ToString(props.Item.ID),
	},
		fixtureButton("fixture-list-select-"+gf.ToString(props.Item.ID), "Select "+gf.ToString(props.Item.ID), func() {
			props.OnSelect(props.Item.ID)
		}),
		fixtureButton("fixture-list-toggle-"+gf.ToString(props.Item.ID), "Toggle "+gf.ToString(props.Item.ID), func() {
			props.OnToggle(props.Item.ID)
		}),
	)
}

func renderFixtureTableRow(props fixtureItemProps) gf.Node {
	useFixtureItemLifetime(props.Kind, props.Item.ID)
	className := "fixture-table-row"
	if props.Selected {
		className += " selected"
	}
	if props.Toggled {
		className += " toggled"
	}
	return gf.El("tr", gf.Props{
		"class":       className,
		"style":       props.Style,
		"data-testid": "fixture-table-row-" + gf.ToString(props.Item.ID),
	},
		gf.El("td", gf.Props{}, fixtureButton("fixture-table-select-"+gf.ToString(props.Item.ID), "Select "+gf.ToString(props.Item.ID), func() {
			props.OnSelect(props.Item.ID)
		})),
		gf.El("td", gf.Props{}, fixtureButton("fixture-table-toggle-"+gf.ToString(props.Item.ID), "Toggle "+gf.ToString(props.Item.ID), func() {
			props.OnToggle(props.Item.ID)
		})),
	)
}

func useFixtureItemLifetime(kind string, id int) {
	gf.UseEffect(func() gf.Cleanup {
		fixtureIncrementMap("mounts", fixtureItemKey(kind, id))
		return func() {
			fixtureIncrementMap("cleanups", fixtureItemKey(kind, id))
		}
	})
}

func fixtureItems(length int) []fixtureItem {
	items := make([]fixtureItem, length)
	for index := range items {
		items[index] = fixtureItem{ID: index}
	}
	return items
}

func fixtureButton(testID string, label string, click func()) gf.Node {
	return gf.El("button", gf.Props{
		"type":        "button",
		"data-testid": testID,
		"OnClick":     click,
	}, gf.Text(label))
}

func installFixtureAudit() {
	object := js.Global().Get("Object").New()
	object.Set("appMounts", 0)
	object.Set("appCleanups", 0)
	object.Set("appRenders", 0)
	object.Set("mounts", js.Global().Get("Object").New())
	object.Set("cleanups", js.Global().Get("Object").New())
	object.Set("interactions", js.Global().Get("Array").New())
	object.Set("runtimeErrors", js.Global().Get("Array").New())
	js.Global().Set("__virtualRangeFixture", object)
	gf.SetErrorHandler(func(info gf.ErrorInfo) {
		object.Get("runtimeErrors").Call("push", info.Phase.String()+":"+info.Component+":"+info.Operation)
	})
}

func fixtureIncrementScalar(name string) {
	audit := js.Global().Get("__virtualRangeFixture")
	audit.Set(name, audit.Get(name).Int()+1)
}

func fixtureIncrementMap(name string, key string) {
	bucket := js.Global().Get("__virtualRangeFixture").Get(name)
	value := bucket.Get(key)
	count := 0
	if value.Type() == js.TypeNumber {
		count = value.Int()
	}
	bucket.Set(key, count+1)
}

func fixtureRecordInteraction(kind string, action string, id int) {
	js.Global().Get("__virtualRangeFixture").Get("interactions").Call(
		"push",
		kind+":"+action+":"+gf.ToString(id),
	)
}

func fixtureItemKey(kind string, id int) string {
	return kind + ":" + gf.ToString(id)
}
