package utils

// Pointer returns a pointer to a value of any type.
func Pointer[T any](v T) *T {
	return &v
}
