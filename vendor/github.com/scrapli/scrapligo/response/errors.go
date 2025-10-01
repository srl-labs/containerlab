package response

import "fmt"

// OperationError is an error object returned when a scrapli operation completes "successfully" --
// as in does not have an EOF/timeout or otherwise unrecoverable error -- but contains output in the
// device's response indicating that an input was bad/invalid or device failed to process it at
// that time.
type OperationError struct {
	Input       string
	Output      string
	ErrorString string
}

// Error returns an error string for the OperationError object.
func (e *OperationError) Error() string {
	return fmt.Sprintf(
		"operation error from input '%s'. matched error sub-string '%s'. full output: '%s'",
		e.Input,
		e.ErrorString,
		e.Output,
	)
}

// MultiOperationError is an error object for scrapli *multi* operations.
type MultiOperationError struct {
	Operations []*OperationError
}

// Error returns an error string for the MultiOperationError object.
func (e *MultiOperationError) Error() string {
	if len(e.Operations) == 1 {
		return fmt.Sprintf(
			"operation error from input '%s'. matched error sub-string '%s'. full output: '%s'",
			e.Operations[0].Input,
			e.Operations[0].ErrorString,
			e.Operations[0].Output,
		)
	}

	return fmt.Sprintf(
		"operation error from multiple inputs. %d indicated errors",
		len(e.Operations),
	)
}
