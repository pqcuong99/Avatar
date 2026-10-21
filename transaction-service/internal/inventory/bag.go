// Package inventory is the in-memory model of one character's bag (TH-02,
// docs/02 §2). It is loaded inside a database transaction, changed by the
// business rules, then written back; it never touches the database itself.
//
// Rules:
//   - The bag has Cap numbered slots; a slot holds one stack of one item.
//   - A stackable item fills its existing stacks first (lowest slot number
//     first), then empty slots in increasing order. Stacks never exceed the
//     catalog stack_limit (999 for farm, upgrade and worm items).
//   - Instance items (fashion) take one slot each and never stack.
//   - Add is all or nothing: if the whole quantity does not fit, nothing
//     changes. Callers use CanFit to decide between bag and pending.
package inventory

import (
	"errors"
	"sort"
)

// ErrNoSpace means the requested quantity does not fit as a whole.
var ErrNoSpace = errors.New("inventory: not enough space")

// ErrNotEnough means the bag holds less than the requested quantity.
var ErrNotEnough = errors.New("inventory: not enough items")

type Item struct {
	Code       string
	StackLimit int
	Instance   bool // fashion: one item_instance per slot
}

type Slot struct {
	No         int
	ItemCode   string
	Quantity   int
	InstanceID string // empty for stackable items
}

type Bag struct {
	Cap   int
	slots map[int]*Slot
	dirty map[int]bool
}

func New(capacity int, slots []Slot) *Bag {
	b := &Bag{Cap: capacity, slots: map[int]*Slot{}, dirty: map[int]bool{}}
	for i := range slots {
		s := slots[i]
		b.slots[s.No] = &s
	}
	return b
}

// Slots returns the occupied slots ordered by slot number.
func (b *Bag) Slots() []Slot {
	out := make([]Slot, 0, len(b.slots))
	for _, s := range b.slots {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].No < out[j].No })
	return out
}

// Count is the total quantity of an item held in the bag.
func (b *Bag) Count(code string) int64 {
	var n int64
	for _, s := range b.slots {
		if s.ItemCode == code {
			n += int64(s.Quantity)
		}
	}
	return n
}

func (b *Bag) emptySlots() int {
	n := 0
	for no := 1; no <= b.Cap; no++ {
		if _, used := b.slots[no]; !used {
			n++
		}
	}
	return n
}

// FreeCapacity is how many more units of item fit right now: room left in
// its existing stacks plus a full stack per empty slot.
func (b *Bag) FreeCapacity(item Item) int64 {
	if item.Instance {
		return int64(b.emptySlots())
	}
	var free int64
	for _, s := range b.slots {
		if s.ItemCode == item.Code && s.Quantity < item.StackLimit {
			free += int64(item.StackLimit - s.Quantity)
		}
	}
	return free + int64(b.emptySlots())*int64(item.StackLimit)
}

func (b *Bag) CanFit(item Item, quantity int64) bool {
	return quantity <= b.FreeCapacity(item)
}

// Add puts quantity units of a stackable item into the bag, all or nothing.
func (b *Bag) Add(item Item, quantity int64) error {
	if item.Instance {
		return errors.New("inventory: use AddInstance for instance items")
	}
	if quantity <= 0 {
		return errors.New("inventory: quantity must be positive")
	}
	if !b.CanFit(item, quantity) {
		return ErrNoSpace
	}
	left := quantity
	for _, no := range b.sortedSlotNos() {
		s := b.slots[no]
		if left == 0 {
			break
		}
		if s.ItemCode != item.Code || s.Quantity >= item.StackLimit {
			continue
		}
		take := min(left, int64(item.StackLimit-s.Quantity))
		s.Quantity += int(take)
		left -= take
		b.dirty[no] = true
	}
	for no := 1; no <= b.Cap && left > 0; no++ {
		if _, used := b.slots[no]; used {
			continue
		}
		take := min(left, int64(item.StackLimit))
		b.slots[no] = &Slot{No: no, ItemCode: item.Code, Quantity: int(take)}
		left -= take
		b.dirty[no] = true
	}
	return nil
}

// AddInstance puts one instance item into the lowest empty slot and returns
// the slot number.
func (b *Bag) AddInstance(item Item, instanceID string) (int, error) {
	for no := 1; no <= b.Cap; no++ {
		if _, used := b.slots[no]; !used {
			b.slots[no] = &Slot{No: no, ItemCode: item.Code, Quantity: 1, InstanceID: instanceID}
			b.dirty[no] = true
			return no, nil
		}
	}
	return 0, ErrNoSpace
}

// Remove takes quantity units of a stackable item, emptying the highest
// numbered stacks first so the first slots stay stable for the player.
func (b *Bag) Remove(code string, quantity int64) error {
	if quantity <= 0 {
		return errors.New("inventory: quantity must be positive")
	}
	if b.Count(code) < quantity {
		return ErrNotEnough
	}
	nos := b.sortedSlotNos()
	left := quantity
	for i := len(nos) - 1; i >= 0 && left > 0; i-- {
		s := b.slots[nos[i]]
		if s.ItemCode != code || s.InstanceID != "" {
			continue
		}
		take := min(left, int64(s.Quantity))
		s.Quantity -= int(take)
		left -= take
		b.dirty[s.No] = true
		if s.Quantity == 0 {
			delete(b.slots, s.No)
		}
	}
	if left > 0 {
		return ErrNotEnough // only instance slots held the rest
	}
	return nil
}

// RemoveInstance frees the slot holding the given instance.
func (b *Bag) RemoveInstance(instanceID string) error {
	for no, s := range b.slots {
		if s.InstanceID == instanceID {
			delete(b.slots, no)
			b.dirty[no] = true
			return nil
		}
	}
	return ErrNotEnough
}

// FindInstance returns the slot holding an instance.
func (b *Bag) FindInstance(instanceID string) (Slot, bool) {
	for _, s := range b.slots {
		if s.InstanceID == instanceID {
			return *s, true
		}
	}
	return Slot{}, false
}

// Changes lists the slots to write back: current content of changed slots and
// the numbers of slots that became empty.
func (b *Bag) Changes() (upserts []Slot, deletes []int) {
	for no := range b.dirty {
		if s, ok := b.slots[no]; ok {
			upserts = append(upserts, *s)
		} else {
			deletes = append(deletes, no)
		}
	}
	sort.Slice(upserts, func(i, j int) bool { return upserts[i].No < upserts[j].No })
	sort.Ints(deletes)
	return upserts, deletes
}

func (b *Bag) sortedSlotNos() []int {
	nos := make([]int, 0, len(b.slots))
	for no := range b.slots {
		nos = append(nos, no)
	}
	sort.Ints(nos)
	return nos
}
