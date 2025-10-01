package response

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/scrapli/scrapligo/util"
)

const (
	v1Dot0      = "1.0"
	v1Dot1      = "1.1"
	v1Dot0Delim = "]]>]]>"
	xmlHeader   = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"

	// https://datatracker.ietf.org/doc/html/rfc6242#section-4.2 max chunk size is 4294967295 so
	// for us that is a max of 10 chars that the chunk size could be when we are parsing it out of
	// raw bytes.
	maxChunkSizeCharLen = 10
)

var errNetconf1Dot1Error = errors.New("unable to parse netconf 1.1 response")

func errNetconf1Dot1ParseError(msg string) error {
	return fmt.Errorf("%w: %s", errNetconf1Dot1Error, msg)
}

type netconfPatterns struct {
	rpcErrors       *regexp.Regexp
	rpcSingleErrors *regexp.Regexp
}

var (
	netconfPatternsInstance     *netconfPatterns //nolint:gochecknoglobals
	netconfPatternsInstanceOnce sync.Once        //nolint:gochecknoglobals
)

func getNetconfPatterns() *netconfPatterns {
	netconfPatternsInstanceOnce.Do(func() {
		netconfPatternsInstance = &netconfPatterns{
			rpcErrors:       regexp.MustCompile(`(?s)<rpc-errors?>(.*)</rpc-errors?>`),
			rpcSingleErrors: regexp.MustCompile(`(?sU)<rpc-errors?>.*</rpc-errors?>`),
		}
	})

	return netconfPatternsInstance
}

// NewNetconfResponse prepares a new NetconfResponse object.
func NewNetconfResponse(
	input,
	framedInput []byte,
	host string,
	port int,
	version string,
) *NetconfResponse {
	return &NetconfResponse{
		Host:        host,
		Port:        port,
		Input:       input,
		FramedInput: framedInput,
		Result:      "",
		StartTime:   time.Now(),
		EndTime:     time.Time{},
		ElapsedTime: 0,
		FailedWhenContains: [][]byte{
			[]byte("<rpc-error>"),
			[]byte("<rpc-errors>"),
			[]byte("</rpc-error>"),
			[]byte("</rpc-errors>"),
			// for juniper, with set system services netconf rfc-compliant
			[]byte("<nc:rpc-error>"),
			[]byte("</nc:rpc-error>"),
		},
		NetconfVersion: version,
	}
}

// NetconfResponse is a struct returned from all netconf driver operations.
type NetconfResponse struct {
	Host                 string
	Port                 int
	Input                []byte
	FramedInput          []byte
	RawResult            []byte
	Result               string
	StartTime            time.Time
	EndTime              time.Time
	ElapsedTime          float64
	FailedWhenContains   [][]byte
	Failed               error
	StripNamespaces      bool
	NetconfVersion       string
	ErrorMessages        []string
	WarningErrorMessages []string
	SubscriptionID       int
}

// Record records the output of a NETCONF operation.
func (r *NetconfResponse) Record(b []byte) {
	r.EndTime = time.Now()
	r.ElapsedTime = r.EndTime.Sub(r.StartTime).Seconds()

	r.RawResult = b

	if util.ByteContainsAny(r.RawResult, r.FailedWhenContains) {
		patterns := getNetconfPatterns()

		r.Failed = &OperationError{
			Input:       string(r.Input),
			Output:      r.Result,
			ErrorString: string(patterns.rpcErrors.Find(r.RawResult)),
		}

		for _, rpcerr := range patterns.rpcSingleErrors.FindAll(r.RawResult, -1) {
			errStr := string(rpcerr)

			switch {
			case strings.Contains(errStr, "<error-severity>error</error-severity>"):
				r.ErrorMessages = append(r.ErrorMessages, errStr)
			case strings.Contains(errStr, "<error-severity>warning</error-severity>"):
				r.WarningErrorMessages = append(r.WarningErrorMessages, errStr)
			}
		}
	}

	switch r.NetconfVersion {
	case v1Dot0:
		r.record1dot0()
	case v1Dot1:
		r.record1dot1()
	}
}

func (r *NetconfResponse) record1dot0() {
	b := r.RawResult

	b = bytes.TrimPrefix(b, []byte(xmlHeader))
	// trim space before trimming suffix because we usually have a trailing newline!
	b = bytes.TrimSuffix(bytes.TrimSpace(b), []byte(v1Dot0Delim))

	r.Result = string(bytes.TrimSpace(b))
}

func (r *NetconfResponse) record1dot1() {
	err := r.record1dot1Chunks()
	if err != nil {
		r.Failed = &OperationError{
			Input:       string(r.Input),
			Output:      r.Result,
			ErrorString: err.Error(),
		}
	}
}

func (r *NetconfResponse) record1dot1Chunks() error {
	d := bytes.TrimSpace(r.RawResult)

	if len(d) == 0 || d[0] != byte('#') {
		return errNetconf1Dot1ParseError(
			"unable to parse netconf response: no chunk marker at start of data",
		)
	}

	var joined []byte

	var cursor int

	for cursor < len(d) {
		if d[cursor] == byte('\n') {
			// we don't need this at the start of this loop, but this lets us easily handle newlines
			// between chunks
			cursor++

			continue
		}

		if d[cursor] != byte('#') {
			return errNetconf1Dot1ParseError(fmt.Sprintf(
				"unable to parse netconf response: chunk marker missing, got '%s'",
				string(d[cursor])))
		}

		cursor++

		if d[cursor] == byte('#') {
			break
		}

		var chunkSizeStr string

		for chunkSizeLen := 0; chunkSizeLen <= maxChunkSizeCharLen; chunkSizeLen++ {
			if d[cursor+chunkSizeLen] == byte('\n') {
				chunkSizeStr = string(d[cursor : cursor+chunkSizeLen])

				cursor += chunkSizeLen + 1

				break
			}
		}

		if chunkSizeStr == "" {
			return errNetconf1Dot1ParseError(
				"unable to parse netconf response: failed parsing chunk size",
			)
		}

		chunkSize, err := strconv.Atoi(chunkSizeStr)
		if err != nil {
			return errNetconf1Dot1ParseError(
				fmt.Sprintf(
					"unable to parse netconf response: unable to parse chunk size '%s': %s",
					chunkSizeStr,
					err,
				),
			)
		}

		joined = append(joined, d[cursor:cursor+chunkSize]...)

		// obviously no reason to iterate over the chunk we just yoinked out, so increment the
		// cursor accordingly -- we can ignore newlines after the chunk since we handle that at
		// the top of this loop
		cursor += chunkSize
	}

	joined = bytes.TrimPrefix(joined, []byte(xmlHeader))

	r.Result = string(bytes.TrimSpace(joined))

	return nil
}
