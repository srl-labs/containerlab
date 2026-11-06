package capture

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

// Snap length bounds of a capture.
const (
	// DefaultSnapLength is the snap length of a request without one.
	DefaultSnapLength = 256 << 10
	MinSnapLength     = 64
	MaxSnapLength     = 1 << 20
)

// ValidateRequest checks the bounds of a capture request and fills the default snap length.
func ValidateRequest(req *clablabruntime.CaptureRequest) error {
	if req.Name == "" {
		return fmt.Errorf("topology name is required")
	}
	if len(req.Targets) == 0 {
		return fmt.Errorf("at least one capture target (node and interface) is required")
	}
	for _, target := range req.Targets {
		if target.Node == "" || target.Interface == "" {
			return fmt.Errorf("capture target %+v needs a node and an interface", target)
		}
	}
	if req.Duration < 0 {
		return fmt.Errorf("capture duration must not be negative")
	}
	if req.PacketLimit < 0 {
		return fmt.Errorf("capture packet limit must not be negative")
	}
	if req.SnapLength == 0 {
		req.SnapLength = DefaultSnapLength
	}
	if req.SnapLength < MinSnapLength || req.SnapLength > MaxSnapLength {
		return fmt.Errorf("capture snap length %d is outside %d to %d bytes", req.SnapLength,
			MinSnapLength, MaxSnapLength)
	}

	return nil
}

// FindInterface matches a requested interface by its name, then by its alias.
func FindInterface(
	entries []clablabruntime.CaptureInterface,
	name string,
) (clablabruntime.CaptureInterface, bool) {
	for _, entry := range entries {
		if entry.Interface == name {
			return entry, true
		}
	}
	for _, entry := range entries {
		if entry.Alias != "" && entry.Alias == name {
			return entry, true
		}
	}

	return clablabruntime.CaptureInterface{}, false
}

// DescribeInterfaces lists interfaces for an error message: name (alias), ...
func DescribeInterfaces(entries []clablabruntime.CaptureInterface) string {
	if len(entries) == 0 {
		return "none"
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Interface
		if entry.Alias != "" {
			name += " (" + entry.Alias + ")"
		}
		names = append(names, name)
	}

	return strings.Join(names, ", ")
}

// ReportGap hands a gap to onGap, or logs it as a warning when onGap is nil.
func ReportGap(onGap func(clablabruntime.CaptureGap), gap clablabruntime.CaptureGap) {
	if onGap != nil {
		onGap(gap)
		return
	}

	to := "unknown"
	if !gap.To.IsZero() {
		to = gap.To.Format(time.RFC3339Nano)
	}
	log.Warn("Capture has a gap", "node", gap.Node, "interfaces", strings.Join(gap.Interfaces, ","),
		"from", gap.From.Format(time.RFC3339Nano), "to", to, "reason", gap.Reason)
}

// Output is the merged pcapng stream of one capture: the section header with one interface per
// capture target, then the packets of every target in timestamp order (through a Reorder), up
// to an optional packet limit.
type Output struct {
	writer  *Writer
	reorder *Reorder
	limit   int64
	written atomic.Int64
	stop    func()

	cancelRun context.CancelFunc
	runDone   chan struct{}
}

// NewOutput writes the section header and one Interface Description Block per name to w, then
// starts the reorder buffer with the given window (zero or less writes packets as they come).
// stop is called once the packet limit is reached or a write failed. Close must be called.
func NewOutput(
	w io.Writer,
	names []string,
	snapLength, packetLimit int,
	window time.Duration,
	stop func(),
) (*Output, error) {
	writer := NewWriter(w)
	if err := writer.WriteHeader(
		names,
		uint32(snapLength),
	); err != nil { //nolint:gosec // validated
		return nil, err
	}

	out := &Output{
		writer:  writer,
		reorder: NewReorder(window, writer.WritePacket, stop),
		limit:   int64(packetLimit),
		stop:    stop,
		runDone: make(chan struct{}),
	}

	var runCtx context.Context
	runCtx, out.cancelRun = context.WithCancel(context.Background())
	go func() {
		defer close(out.runDone)
		out.reorder.Run(runCtx)
	}()

	return out, nil
}

// Emit hands a packet of the interface with the given index to the stream. It reports false
// once the packet limit is reached and the capture stops, and returns the error of a failed
// write.
func (o *Output) Emit(id uint32, packet *Packet) (bool, error) {
	if o.limit > 0 {
		count := o.written.Add(1)
		if count > o.limit {
			o.stop()
			return false, nil
		}
		if err := o.reorder.Push(id, packet); err != nil {
			return false, err
		}
		if count == o.limit {
			o.stop()
			return false, nil
		}

		return true, nil
	}

	o.written.Add(1)

	return true, o.reorder.Push(id, packet)
}

// Written returns the number of packets handed to the stream so far.
func (o *Output) Written() int64 {
	return o.written.Load()
}

// Hold keeps packets stamped after timestamp back until Release(key).
func (o *Output) Hold(key any, timestamp int64) {
	o.reorder.Hold(key, timestamp)
}

// Release removes the hold of key.
func (o *Output) Release(key any) {
	o.reorder.Release(key)
}

// Close writes the packets still held back and returns the first write error.
func (o *Output) Close() error {
	o.cancelRun()
	<-o.runDone
	o.reorder.Flush()

	return o.writer.Err()
}
