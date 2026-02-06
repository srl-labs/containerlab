package core

// SaveOption is a type used for functional options for the CLab Save method.
type SaveOption func(o *SaveOptions)

// SaveOptions represents the options for saving lab configs.
type SaveOptions struct {
	copyDst string
}

// NewSaveOptions returns a new save options object.
func NewSaveOptions() *SaveOptions {
	return &SaveOptions{}
}

// WithCopyOut sets the destination directory for the copied saved configs.
func WithCopyOut(dst string) SaveOption {
	return func(o *SaveOptions) {
		o.copyDst = dst
	}
}
