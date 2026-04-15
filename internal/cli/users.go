package cli

// csvIndex builds a column-name → index map from a header row.
func csvIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[h] = i
	}
	return m
}

// csvField returns the value at the named column, or "" if absent.
func csvField(record []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(record) {
		return ""
	}
	return record[i]
}
