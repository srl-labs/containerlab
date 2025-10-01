package response

import "github.com/scrapli/scrapligo/response"

// PlatformResponse is a simple response objects that platform implementations return to the Cfg
// instance.
type PlatformResponse struct {
	Result           string
	ScrapliResponses []*response.Response
}
