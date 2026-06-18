package main

import "strconv"

const virtualizedItemCount = 10000
const virtualListHeight = 360
const virtualListItemHeight = 44
const virtualTableHeight = 420
const virtualTableRowHeight = 40
const virtualOverscan = 8

type DemoItem struct {
	ID      int
	Name    string
	Group   string
	Enabled bool
	Score   int
}

type DemoActionKind int

const (
	DemoActionToggle DemoActionKind = iota
	DemoActionReset
)

type DemoAction struct {
	Kind DemoActionKind
	ID   int
}

func makeDemoItems(count int) []DemoItem {
	items := make([]DemoItem, 0, count)
	for index := 0; index < count; index++ {
		id := index + 1
		items = append(items, DemoItem{
			ID:      id,
			Name:    "Item " + paddedNumber(id),
			Group:   groupName(index),
			Enabled: index%3 != 0,
			Score:   (index*37 + 11) % 1000,
		})
	}
	return items
}

func reduceDemoItems(items []DemoItem, action DemoAction) []DemoItem {
	switch action.Kind {
	case DemoActionToggle:
		return toggleDemoItem(items, action.ID)
	case DemoActionReset:
		return makeDemoItems(virtualizedItemCount)
	default:
		return items
	}
}

func toggleDemoItem(items []DemoItem, id int) []DemoItem {
	next := append([]DemoItem(nil), items...)
	for index := range next {
		if next[index].ID == id {
			next[index].Enabled = !next[index].Enabled
			return next
		}
	}
	return items
}

func findDemoItem(items []DemoItem, id int) (DemoItem, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return DemoItem{}, false
}

func groupName(index int) string {
	switch index % 4 {
	case 0:
		return "Alpha"
	case 1:
		return "Beta"
	case 2:
		return "Gamma"
	default:
		return "Delta"
	}
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func paddedNumber(value int) string {
	if value < 10 {
		return "000" + strconv.Itoa(value)
	}
	if value < 100 {
		return "00" + strconv.Itoa(value)
	}
	if value < 1000 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
