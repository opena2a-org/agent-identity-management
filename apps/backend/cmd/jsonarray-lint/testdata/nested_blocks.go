// Fixture: var-slice declarations nested inside if/for/range/switch blocks.
// The lint walks block bodies recursively and should flag all four.
package fixture

func NestedBlocks(condition bool, items []int, _ map[string]int, kind int) {
	if condition {
		var a []string
		_ = a
	}

	for i := 0; i < 3; i++ {
		var b []int
		_ = b
	}

	for range items {
		var c []float64
		_ = c
	}

	switch kind {
	case 1:
		var d []bool
		_ = d
	}
}
