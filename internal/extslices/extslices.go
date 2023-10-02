package extslices

func TransformFunc[S1 ~[]E1, E1, E2 any](s S1, transform func(E1) E2) []E2 {
	results := make([]E2, len(s))
	for i, e := range s {
		results[i] = transform(e)
	}
	return results
}
