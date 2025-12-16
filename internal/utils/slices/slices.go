package slices

import apiconversion "k8s.io/apimachinery/pkg/conversion"

func TransformFunc[S1 ~[]E1, E1, E2 any](s S1, transform func(E1) E2) []E2 {
	results := make([]E2, len(s))
	for i, e := range s {
		results[i] = transform(e)
	}

	return results
}

func TransformWithConversion[E1, E2 any](s []E1, convert func(*E1, *E2, apiconversion.Scope) error) ([]E2, error) {
	results := make([]E2, len(s))
	for i := range s {
		if err := convert(&s[i], &results[i], nil); err != nil {
			return nil, err
		}
	}

	return results, nil
}
