package graph

import "testing"

func neighborList(m map[int][]int) func(int) ([]int, error) {
	return func(n int) ([]int, error) { return m[n], nil }
}

func TestHasPath_Exists(t *testing.T) {
	nb := neighborList(map[int][]int{
		1: {2},
		2: {3},
		3: {},
	})
	ok, err := HasPath(1, 3, nb)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected path 1->2->3")
	}
}

func TestHasPath_NoPath(t *testing.T) {
	nb := neighborList(map[int][]int{
		1: {2},
		2: {},
		3: {},
	})
	ok, err := HasPath(1, 3, nb)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no path from 1 to 3")
	}
}

func TestHasPath_Self(t *testing.T) {
	nb := neighborList(map[int][]int{
		1: {},
	})
	ok, err := HasPath(1, 1, nb)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected self to be reachable (start == target)")
	}
}

func TestHasPath_NeighborError(t *testing.T) {
	nb := func(n int) ([]int, error) {
		return nil, testError{"neighbor error"}
	}
	_, err := HasPath(1, 2, nb)
	if err == nil {
		t.Fatal("expected error from neighbors callback")
	}
}

func TestHasPath_CycleNoPath(t *testing.T) {
	nb := neighborList(map[int][]int{
		1: {2},
		2: {1},
	})
	ok, err := HasPath(1, 3, nb)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no path from 1 to 3 (3 not in cycle)")
	}
}

type testError struct{ msg string }

func (e testError) Error() string { return e.msg }
