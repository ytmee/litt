package graph

// HasPath performs DFS from start following neighbors, returning true
// if target is reachable. Used for cycle detection in issue graphs.
func HasPath(start, target int, neighbors func(int) ([]int, error)) (bool, error) {
	visited := make(map[int]bool)
	stack := []int{start}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if current == target {
			return true, nil
		}
		if visited[current] {
			continue
		}
		visited[current] = true

		ns, err := neighbors(current)
		if err != nil {
			return false, err
		}
		stack = append(stack, ns...)
	}
	return false, nil
}
