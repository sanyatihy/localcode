package main

import (
	"reflect"
	"testing"
)

func TestWindow(t *testing.T) {
	cases := []struct {
		xs   []int
		n    int
		want []int
	}{
		{[]int{1, 2, 3, 4, 5}, 2, []int{4, 5}},
		{[]int{1, 2, 3}, 3, []int{1, 2, 3}},
		{[]int{1, 2, 3}, 9, []int{1, 2, 3}},
		{[]int{1, 2, 3}, 1, []int{3}},
		{[]int{1, 2, 3}, 0, []int{}},
		{[]int{1, 2, 3}, -1, []int{}},
		{[]int{}, 2, []int{}},
	}
	for _, c := range cases {
		if got := Window(c.xs, c.n); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Window(%v, %d) = %v, want %v", c.xs, c.n, got, c.want)
		}
	}
}
