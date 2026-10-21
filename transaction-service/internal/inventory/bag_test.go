package inventory

import (
	"errors"
	"testing"
)

var carrot = Item{Code: "crop_carrot", StackLimit: 999}
var potato = Item{Code: "crop_potato", StackLimit: 999}
var hat = Item{Code: "hat_red", StackLimit: 1, Instance: true}

func TestAddFillsExistingStacksThenEmptySlots(t *testing.T) {
	b := New(10, []Slot{{No: 2, ItemCode: "crop_carrot", Quantity: 990}, {No: 1, ItemCode: "crop_potato", Quantity: 5}})
	if err := b.Add(carrot, 20); err != nil {
		t.Fatal(err)
	}
	got := b.Slots()
	want := []Slot{
		{No: 1, ItemCode: "crop_potato", Quantity: 5},
		{No: 2, ItemCode: "crop_carrot", Quantity: 999},
		{No: 3, ItemCode: "crop_carrot", Quantity: 11},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slot %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestF15ThousandItemsMakeTwoStacks(t *testing.T) {
	b := New(10, nil)
	if err := b.Add(carrot, 1000); err != nil {
		t.Fatal(err)
	}
	s := b.Slots()
	if len(s) != 2 || s[0].Quantity != 999 || s[1].Quantity != 1 {
		t.Fatalf("got %+v", s)
	}
}

func TestF16InstancesNeverStack(t *testing.T) {
	b := New(10, nil)
	if _, err := b.AddInstance(hat, "i-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AddInstance(hat, "i-2"); err != nil {
		t.Fatal(err)
	}
	s := b.Slots()
	if len(s) != 2 || s[0].InstanceID != "i-1" || s[1].InstanceID != "i-2" {
		t.Fatalf("got %+v", s)
	}
	if b.FreeCapacity(hat) != 8 {
		t.Fatalf("free %d", b.FreeCapacity(hat))
	}
}

func TestAddIsAllOrNothing(t *testing.T) {
	// F-04 relies on this: 120 does not fit into 50 free units -> nothing added.
	slots := []Slot{}
	for no := 1; no <= 9; no++ {
		slots = append(slots, Slot{No: no, ItemCode: "crop_potato", Quantity: 999})
	}
	slots = append(slots, Slot{No: 10, ItemCode: "crop_carrot", Quantity: 949})
	b := New(10, slots)
	if b.FreeCapacity(carrot) != 50 {
		t.Fatalf("free %d", b.FreeCapacity(carrot))
	}
	if err := b.Add(carrot, 120); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("want ErrNoSpace, got %v", err)
	}
	if up, del := b.Changes(); len(up) != 0 || len(del) != 0 {
		t.Fatalf("bag changed: %+v %+v", up, del)
	}
	if b.Count("crop_carrot") != 949 {
		t.Fatal("count changed")
	}
}

func TestRemoveTakesHighestSlotsFirstAndFreesThem(t *testing.T) {
	b := New(10, []Slot{
		{No: 1, ItemCode: "seed_carrot", Quantity: 999},
		{No: 4, ItemCode: "seed_carrot", Quantity: 3},
	})
	if err := b.Remove("seed_carrot", 5); err != nil {
		t.Fatal(err)
	}
	s := b.Slots()
	if len(s) != 1 || s[0].No != 1 || s[0].Quantity != 997 {
		t.Fatalf("got %+v", s)
	}
	up, del := b.Changes()
	if len(up) != 1 || up[0].No != 1 || len(del) != 1 || del[0] != 4 {
		t.Fatalf("changes %+v %+v", up, del)
	}
	if err := b.Remove("seed_carrot", 998); !errors.Is(err, ErrNotEnough) {
		t.Fatalf("want ErrNotEnough, got %v", err)
	}
}

func TestFreeCapacityCountsPartialStacksOfSameItemOnly(t *testing.T) {
	b := New(3, []Slot{
		{No: 1, ItemCode: "crop_carrot", Quantity: 900},
		{No: 2, ItemCode: "crop_potato", Quantity: 1},
	})
	if got := b.FreeCapacity(carrot); got != 99+999 {
		t.Fatalf("carrot free %d", got)
	}
	if got := b.FreeCapacity(potato); got != 998+999 {
		t.Fatalf("potato free %d", got)
	}
}
