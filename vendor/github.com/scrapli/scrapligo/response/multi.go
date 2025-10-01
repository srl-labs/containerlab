package response

import (
	"strings"
	"time"
)

// NewMultiResponse creates a new MultiResponse object.
func NewMultiResponse(host string) *MultiResponse {
	r := &MultiResponse{
		Host:        host,
		StartTime:   time.Now(),
		EndTime:     time.Time{},
		ElapsedTime: 0,
	}

	return r
}

// MultiResponse defines a response object for plural operations -- contains a slice of `Responses`
// for each operation.
type MultiResponse struct {
	Host        string
	StartTime   time.Time
	EndTime     time.Time
	ElapsedTime float64
	Responses   []*Response
	Failed      error
}

// AppendResponse appends a response to the `Responses` attribute of the MultiResponse object.
func (mr *MultiResponse) AppendResponse(r *Response) {
	mr.EndTime = time.Now()
	mr.ElapsedTime = r.EndTime.Sub(r.StartTime).Seconds()

	re, _ := r.Failed.(*OperationError)

	if re != nil {
		if mr.Failed == nil {
			mr.Failed = &MultiOperationError{}
		}

		e, ok := mr.Failed.(*MultiOperationError)
		if ok {
			e.Operations = append(e.Operations, re)
		}
	}

	mr.Responses = append(mr.Responses, r)
}

// JoinedResult is a convenience method to print out a single unified/joined result -- this is
// basically all the channel output for each individual response in the MultiResponse joined by
// newline characters.
func (mr *MultiResponse) JoinedResult() string {
	resultsSlice := make([]string, len(mr.Responses))

	for idx, resp := range mr.Responses {
		resultsSlice[idx] = resp.Result
	}

	return strings.Join(resultsSlice, "\n")
}
