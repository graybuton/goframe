package documentstate

import (
	"errors"
	"strings"
)

// State is one title and description pair selected by an owner.
type State struct {
	Title       string
	Description string
}

// Snapshot describes the currently selected document state.
type Snapshot struct {
	State       State
	ActiveOwner string
	HasOwner    bool
}

// Change identifies one ownership-model transition.
type Change uint8

const (
	ChangeNone Change = iota
	ChangeAdded
	ChangeUpdated
	ChangeRemoved
)

// String returns the stable fixture evidence name for a change.
func (change Change) String() string {
	switch change {
	case ChangeAdded:
		return "added"
	case ChangeUpdated:
		return "updated"
	case ChangeRemoved:
		return "removed"
	default:
		return "none"
	}
}

// Transition is the selected state after one model operation.
type Transition struct {
	Snapshot Snapshot
	Change   Change
	Owner    string
}

type ownerRecord struct {
	key   string
	state State
}

// Coordinator keeps owner priority in mount order while allowing in-place
// owner updates.
type Coordinator struct {
	baseline State
	owners   []ownerRecord
	index    map[string]int
}

// New creates a coordinator with an authored document baseline.
func New(baseline State) *Coordinator {
	return &Coordinator{
		baseline: baseline,
		index:    make(map[string]int),
	}
}

// Snapshot returns the currently selected owner state or the baseline.
func (coordinator *Coordinator) Snapshot() Snapshot {
	if coordinator == nil || len(coordinator.owners) == 0 {
		if coordinator == nil {
			return Snapshot{}
		}
		return Snapshot{State: coordinator.baseline}
	}
	active := coordinator.owners[len(coordinator.owners)-1]
	return Snapshot{
		State:       active.state,
		ActiveOwner: active.key,
		HasOwner:    true,
	}
}

// Set adds an owner or updates its desired state without changing its priority.
func (coordinator *Coordinator) Set(owner string, state State) (Transition, error) {
	if err := validateOwner(owner); err != nil {
		return Transition{}, err
	}
	if coordinator == nil {
		return Transition{}, errors.New("document-state coordinator is nil")
	}
	if index, ok := coordinator.index[owner]; ok {
		if coordinator.owners[index].state == state {
			return coordinator.transition(owner, ChangeNone), nil
		}
		coordinator.owners[index].state = state
		return coordinator.transition(owner, ChangeUpdated), nil
	}
	coordinator.index[owner] = len(coordinator.owners)
	coordinator.owners = append(coordinator.owners, ownerRecord{
		key:   owner,
		state: state,
	})
	return coordinator.transition(owner, ChangeAdded), nil
}

// Remove releases an owner. Removing a non-top owner leaves the selected state
// unchanged.
func (coordinator *Coordinator) Remove(owner string) (Transition, error) {
	if err := validateOwner(owner); err != nil {
		return Transition{}, err
	}
	if coordinator == nil {
		return Transition{}, errors.New("document-state coordinator is nil")
	}
	index, ok := coordinator.index[owner]
	if !ok {
		return coordinator.transition(owner, ChangeNone), nil
	}
	delete(coordinator.index, owner)
	copy(coordinator.owners[index:], coordinator.owners[index+1:])
	coordinator.owners = coordinator.owners[:len(coordinator.owners)-1]
	for next := index; next < len(coordinator.owners); next++ {
		coordinator.index[coordinator.owners[next].key] = next
	}
	return coordinator.transition(owner, ChangeRemoved), nil
}

// OwnerKeys returns owner keys in deterministic priority order.
func (coordinator *Coordinator) OwnerKeys() []string {
	if coordinator == nil {
		return nil
	}
	keys := make([]string, len(coordinator.owners))
	for index, owner := range coordinator.owners {
		keys[index] = owner.key
	}
	return keys
}

// OwnerCount reports the number of active owner records.
func (coordinator *Coordinator) OwnerCount() int {
	if coordinator == nil {
		return 0
	}
	return len(coordinator.owners)
}

func (coordinator *Coordinator) transition(owner string, change Change) Transition {
	return Transition{
		Snapshot: coordinator.Snapshot(),
		Change:   change,
		Owner:    owner,
	}
}

func validateOwner(owner string) error {
	if owner == "" || strings.TrimSpace(owner) == "" {
		return errors.New("document-state owner key must not be empty")
	}
	return nil
}
