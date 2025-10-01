package transport

import (
	"io"
	"os"
)

const (
	// FileTransport transport name.
	FileTransport = "file"
)

// NewFileTransport returns an instance of File transport. This is for testing purposes only.
func NewFileTransport() (*File, error) {
	t := &File{
		fd: nil,
	}

	return t, nil
}

// File transport is a transport object that "connects" to a file rather than a device, it probably
// has no use outside of testing.
type File struct {
	F  string
	fd *os.File

	content []byte

	Writes [][]byte
}

// Open opens the File transport.
func (t *File) Open(a *Args) error {
	_ = a

	f, err := os.Open(t.F)
	if err != nil {
		return err
	}

	t.fd = f

	t.content, err = io.ReadAll(f)
	if err != nil {
		return err
	}

	_ = t.Close()

	return nil
}

// Close is a noop for the File transport.
func (t *File) Close() error {
	return nil
}

// IsAlive always returns true for File transport.
func (t *File) IsAlive() bool {
	return true
}

// Read reads n bytes from the transport. File transport ignores EOF errors, see comment below.
func (t *File) Read(_ int) ([]byte, error) {
	if len(t.content) == 0 {
		// we can just sleep here as this is getting called from a goroutine anyway, by blocking
		// we will stop subsequent reads which means less things for race detector to look at.
		// it seems to be *moderately* successful in speeding up tests in race mode. we need this
		// because in unit tests we read *one* byte at a time -- some test data contains >10k bytes
		// which causes a lot of locks/unlocks and such which makes the race detector have a party.
		select {}
	}

	b := t.content[0]

	t.content = t.content[1:]

	return []byte{b}, nil
}

// Write writes bytes b to the transport.
func (t *File) Write(b []byte) error {
	t.Writes = append(t.Writes, b)

	return nil
}
