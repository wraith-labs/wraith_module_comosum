package misc

// Simple function which gets rid of error return values from functions when
// we know they definitely won't error. If they error anyway, panic.
func NoError[T any](value T, err error) T {
	if err != nil {
		panic("cannot discard non-nil error")
	}
	return value
}
