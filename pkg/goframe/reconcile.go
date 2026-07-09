package goframe

const noChildMatch = -1

// unwrapNode separates reconciliation identity from the renderable node. Keys
// remain available to the mounted tree while the public KeyedNode stays a
// transparent wrapper.
func unwrapNode(node Node) (string, Node) {
	key := ""
	if node == nil {
		return key, Empty()
	}
	for {
		keyed, ok := node.(KeyedNode)
		if !ok {
			return key, node
		}
		if key == "" {
			key = keyed.Key
		}
		node = keyed.Node
		if node == nil {
			return key, Empty()
		}
	}
}

func sameNodeIdentity(oldNode, newNode Node) bool {
	_, oldNode = unwrapNode(oldNode)
	_, newNode = unwrapNode(newNode)

	switch oldNode := oldNode.(type) {
	case VNode:
		newNode, ok := newNode.(VNode)
		return ok && oldNode.Tag == newNode.Tag
	case TextNode:
		_, ok := newNode.(TextNode)
		return ok
	case FragmentNode:
		_, ok := newNode.(FragmentNode)
		return ok
	case EmptyNode:
		_, ok := newNode.(EmptyNode)
		return ok
	case ComponentNode:
		newNode, ok := newNode.(ComponentNode)
		return ok && nodeComponentIdentity(oldNode) == nodeComponentIdentity(newNode)
	default:
		return false
	}
}

// matchChildIndices maps every new child to an old child. Keyed children match
// by key; unkeyed children match by their relative unkeyed position.
func matchChildIndices(oldKeys, newKeys []string) []int {
	matches := make([]int, len(newKeys))
	for index := range matches {
		matches[index] = noChildMatch
	}

	keyed := make(map[string]int, len(oldKeys))
	unkeyed := make([]int, 0, len(oldKeys))
	for index, key := range oldKeys {
		if key == "" {
			unkeyed = append(unkeyed, index)
			continue
		}
		if _, exists := keyed[key]; !exists {
			keyed[key] = index
		}
	}

	used := make([]bool, len(oldKeys))
	nextUnkeyed := 0
	for newIndex, key := range newKeys {
		if key != "" {
			oldIndex, exists := keyed[key]
			if exists && !used[oldIndex] {
				matches[newIndex] = oldIndex
				used[oldIndex] = true
			}
			continue
		}
		for nextUnkeyed < len(unkeyed) && used[unkeyed[nextUnkeyed]] {
			nextUnkeyed++
		}
		if nextUnkeyed < len(unkeyed) {
			oldIndex := unkeyed[nextUnkeyed]
			matches[newIndex] = oldIndex
			used[oldIndex] = true
			nextUnkeyed++
		}
	}
	return matches
}

func stableChildPlacementStart(matches []int) int {
	stableStart := len(matches)
	for index := len(matches) - 1; index > 0; index-- {
		current := matches[index]
		previous := matches[index-1]
		if current == noChildMatch || previous == noChildMatch || previous >= current {
			break
		}
		stableStart = index - 1
	}
	if stableStart == 0 {
		return len(matches)
	}
	return stableStart
}

func stableChildPlacements(matches []int, keys []string) []bool {
	ends := make([]int, len(matches))
	previous := make([]int, len(matches))
	bestIndex := noChildMatch
	bestLength := 0
	length := 0

	for index, match := range matches {
		if match == noChildMatch || keys[index] == "" {
			continue
		}
		low := 0
		high := length
		for low < high {
			mid := (low + high) / 2
			if matches[ends[mid]] < match {
				low = mid + 1
				continue
			}
			high = mid
		}

		previous[index] = noChildMatch
		if low > 0 {
			previous[index] = ends[low-1]
		}
		ends[low] = index
		if low == length {
			length++
		}
		currentLength := low + 1
		if currentLength > bestLength {
			bestLength = currentLength
			bestIndex = index
		}
	}
	if bestIndex == noChildMatch {
		return nil
	}

	stable := make([]bool, len(matches))
	for bestIndex != noChildMatch {
		stable[bestIndex] = true
		bestIndex = previous[bestIndex]
	}
	return stable
}
