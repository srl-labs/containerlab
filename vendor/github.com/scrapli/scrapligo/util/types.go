package util

// Option is a simple type that accepts an interface and returns an error; this should only be used
// in the context of applying options to an object.
type Option func(interface{}) error

// PayloadTestCase is a simple struct used in testing only.
type PayloadTestCase struct {
	Description string
	PayloadFile string
	ExpectErr   bool
}
