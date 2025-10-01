package netconf

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

type message struct {
	XMLName   xml.Name    `xml:"rpc"`
	Namespace string      `xml:"xmlns,attr"`
	MessageID int         `xml:"message-id,attr"`
	Payload   interface{} `xml:",innerxml"`
}

type serializedInput struct {
	rawXML    []byte
	framedXML []byte
}

func (m *message) serialize(
	v string,
	forceSelfClosingTags,
	excludeHeader bool,
) (*serializedInput, error) {
	serialized := &serializedInput{}

	msg, err := xml.Marshal(m)
	if err != nil {
		return nil, err
	}

	if !excludeHeader {
		msg = append([]byte(xmlHeader), msg...)
	}

	if forceSelfClosingTags {
		msg = ForceSelfClosingTags(msg)
	}

	// copy the raw xml (without the netconf framing) before setting up framing bits
	serialized.rawXML = make([]byte, len(msg))
	copy(serialized.rawXML, msg)

	switch v {
	case V1Dot0:
		msg = append(msg, []byte(v1Dot0Delim)...)
	case V1Dot1:
		msg = append([]byte(fmt.Sprintf("#%d\n", len(msg))), msg...)
		msg = append(msg, []byte("\n##")...)
	}

	serialized.framedXML = msg

	return serialized, nil
}

// ForceSelfClosingTags accepts a netconf looking xml byte slice and forces any "empty" tags (tags
// without attributes) to use self-closing tags. For example:
//
// `<running> </running>`
//
// Would be converted to:
//
// `<running/>`.
//
// Ideally this functino would just be replaced with this: https://github.com/golang/go/issues/59710
// but for now this is more preferred than having either a different regex library imported, or
// importing @ECUST_XX proposed package. Historically, this was simply a regex.ReplaceAll but, there
// are/were issues with the pattern accidentally matching over already self closed tags inside
// other tags, for example:
//
// `<target><candidate xyz/></target>`
//
// Would end up like:
//
// `<target><candidate xyz//>
//
// Which is obviously not correct! This could be pretty easily solved in regex with backtracking/
// capture groups but since we don't get that in std library go regex we can instead just write some
// simple code to iterate over stuff and replace as needed after comparing the open/close tags that
// our pattern found actually do match.
func ForceSelfClosingTags(b []byte) []byte {
	ncPatterns := getNetconfPatterns()

	for _, sm := range ncPatterns.emptyTags.FindAllSubmatch(b, -1) {
		fullMatch := sm[0]
		openingTag := sm[1]
		openingTagContents := sm[2]
		closingTag := sm[3]

		if !bytes.Equal(openingTag, closingTag) {
			// we found a chunk that contains an already "self closed" tag, ignore this
			continue
		}

		b = bytes.ReplaceAll(
			b,
			fullMatch,
			[]byte(fmt.Sprintf("<%s%s/>", openingTag, openingTagContents)),
		)
	}

	return b
}
