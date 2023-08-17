package testkit

type (
	// Opt is a wildcard type for any functional options structure.
	Opt any

	// OptFunc is a functional option type.
	// Read more about this pattern: https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
	OptFunc func(opt Opt)
)
