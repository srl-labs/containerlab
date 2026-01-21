package core

// SaveOption is a type used for functional options for the CLab Save method.
type SaveOption func(o *SaveOptions)

// SaveOptions represents the options for saving lab configs.
type SaveOptions struct {
	dst string
}

// NewSaveOptions returns a new save options object.
func NewSaveOptions() *SaveOptions {
	return &SaveOptions{}
}

// WithSaveDst sets the destination directory for saved configs.
func WithSaveDst(dst string) SaveOption {
	return func(o *SaveOptions) {
		o.dst = dst
	}
}
