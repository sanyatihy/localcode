package main

// Window returns the last n elements of xs. When n exceeds len(xs) it returns
// all of them, and n <= 0 returns an empty slice.
func Window(xs []int, n int) []int {
	if n <= 0 {
		return []int{}
	}
	if n > len(xs) {
		n = len(xs)
	}
	return xs[len(xs)-n+1:]
}
