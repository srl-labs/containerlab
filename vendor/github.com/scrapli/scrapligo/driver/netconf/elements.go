package netconf

import (
	"encoding/xml"
	"fmt"

	"github.com/scrapli/scrapligo/util"
)

const (
	reportAll       = "report-all"
	reportAllTagged = "report-all-tagged"
	trim            = "trim"
	explicit        = "explicit"

	// FilterSubtree is a constant representing the subtree filter type.
	FilterSubtree = "subtree"

	// FilterXpath is a constant representing the xpath filter type.
	FilterXpath = "xpath"
)

type sourceElement struct {
	XMLName xml.Name
}

type sourceT struct {
	XMLName xml.Name       `xml:"source"`
	Source  *sourceElement `xml:""`
}

func (d *Driver) buildSourceElem(source string) *sourceT {
	sourceElem := &sourceT{
		XMLName: xml.Name{},
		Source:  &sourceElement{XMLName: xml.Name{Local: source}},
	}

	return sourceElem
}

type targetElement struct {
	XMLName xml.Name
}

type targetT struct {
	XMLName xml.Name       `xml:"target"`
	Source  *targetElement `xml:""`
}

func (d *Driver) buildTargetElem(target string) *targetT {
	targetElem := &targetT{
		XMLName: xml.Name{},
		Source:  &targetElement{XMLName: xml.Name{Local: target}},
	}

	return targetElem
}

type defaultType struct {
	XMLName   xml.Name `xml:"with-defaults"`
	Namespace string   `xml:"xmlns,attr"`
	Type      string   `xml:",innerxml"`
}

func (d *Driver) buildDefaultsElem(defaultsType string) (*defaultType, error) {
	if defaultsType == "" {
		return nil, nil
	}

	switch defaultsType {
	case reportAll, reportAllTagged, trim, explicit:
	default:
		return nil, fmt.Errorf("%w: unknown default type '%s'", util.ErrNetconfError, defaultsType)
	}

	return &defaultType{
		XMLName:   xml.Name{},
		Namespace: defaultNamespace,
		Type:      defaultsType,
	}, nil
}

type filterT struct {
	XMLName xml.Name `xml:"filter"`
	Type    string   `xml:"type,attr"`
	Select  string   `xml:"select,attr,omitempty"`
	Payload string   `xml:",innerxml"`
}

func (d *Driver) buildFilterElem(filter, filterType string) (*filterT, error) {
	if filter == "" || filterType == "" {
		return nil, nil
	}

	var f *filterT

	var err error

	switch filterType {
	case FilterSubtree:
		f = &filterT{
			XMLName: xml.Name{},
			Type:    filterType,
			Select:  "",
			Payload: filter,
		}
	case FilterXpath:
		f = &filterT{
			XMLName: xml.Name{},
			Type:    filterType,
			Select:  filter,
		}
	default:
		err = fmt.Errorf("%w: unknown filter type '%s'", util.ErrNetconfError, filterType)
	}

	return f, err
}
