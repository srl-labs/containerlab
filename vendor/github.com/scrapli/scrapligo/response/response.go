package response

import (
	"time"

	"github.com/scrapli/scrapligo/util"
)

// NewResponse prepares a new Response object.
func NewResponse(
	input,
	host string,
	port int,
	failedWhenContains []string,
) *Response {
	return &Response{
		Host:               host,
		Port:               port,
		Input:              input,
		Result:             "",
		StartTime:          time.Now(),
		EndTime:            time.Time{},
		ElapsedTime:        0,
		FailedWhenContains: failedWhenContains,
	}
}

// Response is a struct returned from most (all?) generic and network driver "single" operations.
type Response struct {
	Host               string
	Port               int
	Input              string
	RawResult          []byte
	Result             string
	StartTime          time.Time
	EndTime            time.Time
	ElapsedTime        float64
	FailedWhenContains []string
	// Failed returns an error if any of the `FailedWhenContains` substrings are seen in the output
	// returned from the device. This error indicates that the operation has completed successfully,
	// but that an input was bad/invalid or device failed to process it at that time
	Failed error
}

// Record records the output of an operation.
func (r *Response) Record(b []byte) {
	r.EndTime = time.Now()
	r.ElapsedTime = r.EndTime.Sub(r.StartTime).Seconds()

	r.RawResult = b
	r.Result = string(b)

	s := util.StringContainsAnySubStrs(r.Result, r.FailedWhenContains)

	if s != "" {
		r.Failed = &OperationError{
			Input:       r.Input,
			Output:      r.Result,
			ErrorString: s,
		}
	}
}

// TextFsmParse parses recorded output w/ a provided textfsm template.
// the argument is interpreted as URL or filesystem path, for example:
// response.TextFsmParse("http://example.com/textfsm.template") or
// response.TextFsmParse("./local/textfsm.template").
func (r *Response) TextFsmParse(path string) ([]map[string]interface{}, error) {
	return util.TextFsmParse(r.Result, path)
}
