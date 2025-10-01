package response

import (
	"time"

	"github.com/scrapli/scrapligo/response"
)

// NewResponse returns a new Response object with the StartTime set to now.
func NewResponse(op, host string) *Response {
	return &Response{
		Op:        op,
		Host:      host,
		StartTime: time.Now(),
		EndTime:   time.Time{},
	}
}

// Response is similar to the scrapligo response.Response object, but is tailored for scrapligocfg
// uses. The Result field is the primary result of the given operation -- for example if the
// operation was GetVersion, then the Result is the parsed version string. The Op field is the
// human-readable name of the operation.
type Response struct {
	Result string
	Op     string

	Host        string
	StartTime   time.Time
	EndTime     time.Time
	ElapsedTime float64

	ScrapliResponses []*response.Response

	Failed error
}

// Record "records" the slice of scrapligo response.Response objects and the final result of the
// scrapligocfg operation.
func (r *Response) Record(rs []*response.Response, result string) {
	r.EndTime = time.Now()
	r.ElapsedTime = r.EndTime.Sub(r.StartTime).Seconds()

	r.Result = result

	r.ScrapliResponses = append(r.ScrapliResponses, rs...)

	for _, sr := range rs {
		if sr.Failed != nil {
			r.Failed = sr.Failed

			break
		}
	}
}
