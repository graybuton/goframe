package documentmeta

import (
	"errors"
	"strings"
)

// Metadata is one title and description pair.
type Metadata struct {
	Title       string
	Description string
}

// Change identifies one ownership transition.
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

// Owner is an opaque ownership lifetime allocated by one Coordinator.
type Owner struct {
	coordinator *Coordinator
	id          uint64
}

// ID returns the fixture-local identity used only for deterministic evidence.
func (owner *Owner) ID() uint64 {
	if owner == nil {
		return 0
	}
	return owner.id
}

// Snapshot is the currently selected metadata and owner.
type Snapshot struct {
	Metadata      Metadata
	ActiveOwnerID uint64
	HasOwner      bool
	OwnerCount    int
}

// Transition describes one coordinator operation.
type Transition struct {
	Snapshot TransitionSnapshot
	Change   Change
	OwnerID  uint64
}

// TransitionSnapshot is kept distinct so transition values remain comparable.
type TransitionSnapshot = Snapshot

type ownerRecord struct {
	owner    *Owner
	metadata Metadata
}

// Statistics records fixture-only ownership lifecycle evidence.
type Statistics struct {
	TokenCreations         int
	CommittedIDAssignments int
	ActiveAdditions        int
	Updates                int
	Releases               int
	ActiveOwnerCount       int
	LastCommittedOwnerID   uint64
}

// Coordinator selects the most recently activated owner while preserving
// existing-owner priority across updates.
type Coordinator struct {
	baseline Metadata
	nextID   uint64
	owners   []ownerRecord
	index    map[*Owner]int
	stats    Statistics
}

// New creates an empty coordinator with an authored baseline.
func New(baseline Metadata) *Coordinator {
	return &Coordinator{
		baseline: baseline,
		index:    make(map[*Owner]int),
	}
}

// NewOwner allocates an inactive ownership lifetime.
func (coordinator *Coordinator) NewOwner() *Owner {
	if coordinator == nil {
		panic("document metadata: owner coordinator is nil")
	}
	coordinator.stats.TokenCreations++
	return &Owner{
		coordinator: coordinator,
	}
}

// Snapshot returns the selected owner metadata or the authored baseline.
func (coordinator *Coordinator) Snapshot() Snapshot {
	if coordinator == nil {
		return Snapshot{}
	}
	if len(coordinator.owners) == 0 {
		return Snapshot{Metadata: coordinator.baseline}
	}
	active := coordinator.owners[len(coordinator.owners)-1]
	return Snapshot{
		Metadata:      active.metadata,
		ActiveOwnerID: active.owner.id,
		HasOwner:      true,
		OwnerCount:    len(coordinator.owners),
	}
}

// Publish activates a new owner or updates an active owner's pair in place.
func (coordinator *Coordinator) Publish(owner *Owner, metadata Metadata) (Transition, error) {
	if err := coordinator.validateOwner(owner); err != nil {
		return Transition{}, err
	}
	if index, ok := coordinator.index[owner]; ok {
		if coordinator.owners[index].metadata == metadata {
			return coordinator.transition(owner, ChangeNone), nil
		}
		coordinator.owners[index].metadata = metadata
		coordinator.stats.Updates++
		return coordinator.transition(owner, ChangeUpdated), nil
	}
	if owner.id == 0 {
		coordinator.nextID++
		owner.id = coordinator.nextID
		coordinator.stats.CommittedIDAssignments++
	}
	coordinator.index[owner] = len(coordinator.owners)
	coordinator.owners = append(coordinator.owners, ownerRecord{
		owner:    owner,
		metadata: metadata,
	})
	coordinator.stats.ActiveAdditions++
	return coordinator.transition(owner, ChangeAdded), nil
}

// Release removes an active owner. Duplicate releases are no-ops.
func (coordinator *Coordinator) Release(owner *Owner) (Transition, error) {
	if err := coordinator.validateOwner(owner); err != nil {
		return Transition{}, err
	}
	index, ok := coordinator.index[owner]
	if !ok {
		return coordinator.transition(owner, ChangeNone), nil
	}
	delete(coordinator.index, owner)
	copy(coordinator.owners[index:], coordinator.owners[index+1:])
	coordinator.owners = coordinator.owners[:len(coordinator.owners)-1]
	for next := index; next < len(coordinator.owners); next++ {
		coordinator.index[coordinator.owners[next].owner] = next
	}
	coordinator.stats.Releases++
	return coordinator.transition(owner, ChangeRemoved), nil
}

// Stats returns a read-only snapshot of fixture ownership lifecycle evidence.
func (coordinator *Coordinator) Stats() Statistics {
	if coordinator == nil {
		return Statistics{}
	}
	stats := coordinator.stats
	stats.ActiveOwnerCount = len(coordinator.owners)
	stats.LastCommittedOwnerID = coordinator.nextID
	return stats
}

// OwnerIDs returns active owners in activation-priority order.
func (coordinator *Coordinator) OwnerIDs() []uint64 {
	if coordinator == nil {
		return nil
	}
	ids := make([]uint64, len(coordinator.owners))
	for index, owner := range coordinator.owners {
		ids[index] = owner.owner.id
	}
	return ids
}

func (coordinator *Coordinator) validateOwner(owner *Owner) error {
	if coordinator == nil {
		return errors.New("document metadata: coordinator is nil")
	}
	if owner == nil {
		return errors.New("document metadata: owner is nil")
	}
	if owner.coordinator != coordinator {
		return errors.New("document metadata: owner belongs to another coordinator")
	}
	return nil
}

func (coordinator *Coordinator) transition(owner *Owner, change Change) Transition {
	return Transition{
		Snapshot: coordinator.Snapshot(),
		Change:   change,
		OwnerID:  owner.id,
	}
}

// StringOwners adapts the current application-local string-key pattern to the
// same opaque-token coordinator used by the three candidate prototypes.
type StringOwners struct {
	coordinator *Coordinator
	owners      map[string]*Owner
}

// NewStringOwners creates the explicit control adapter.
func NewStringOwners(coordinator *Coordinator) *StringOwners {
	return &StringOwners{
		coordinator: coordinator,
		owners:      make(map[string]*Owner),
	}
}

// Publish activates or updates the owner associated with key.
func (owners *StringOwners) Publish(key string, metadata Metadata) (Transition, error) {
	if strings.TrimSpace(key) == "" {
		return Transition{}, errors.New("document metadata: owner key must not be empty")
	}
	if owners == nil || owners.coordinator == nil {
		return Transition{}, errors.New("document metadata: string-owner bindings are missing")
	}
	owner := owners.owners[key]
	if owner == nil {
		owner = owners.coordinator.NewOwner()
		owners.owners[key] = owner
	}
	return owners.coordinator.Publish(owner, metadata)
}

// Release removes the owner associated with key.
func (owners *StringOwners) Release(key string) (Transition, error) {
	if strings.TrimSpace(key) == "" {
		return Transition{}, errors.New("document metadata: owner key must not be empty")
	}
	if owners == nil || owners.coordinator == nil {
		return Transition{}, errors.New("document metadata: string-owner bindings are missing")
	}
	owner := owners.owners[key]
	if owner == nil {
		return Transition{Snapshot: owners.coordinator.Snapshot()}, nil
	}
	transition, err := owners.coordinator.Release(owner)
	if err == nil {
		delete(owners.owners, key)
	}
	return transition, err
}

// Stats returns fixture ownership evidence for the explicit control adapter.
func (owners *StringOwners) Stats() Statistics {
	if owners == nil {
		return Statistics{}
	}
	return owners.coordinator.Stats()
}
