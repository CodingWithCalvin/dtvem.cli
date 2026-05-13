package doctor

// plural returns one or many suffix forms based on count. Inline
// helper so the doctor checks don't pull in a heavier pluralization
// dependency for what amounts to a handful of titles per report.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
