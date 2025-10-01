package util

import (
	"regexp"
	"strings"
)

func cleanResponseMap() map[string]func(r string) string {
	return map[string]func(r string) string{
		"arista_eos":    aristaEosCleanResponse,
		"cisco_iosxr":   ciscoIosxrCleanResponse,
		"cisco_iosxe":   ciscoIosxeCleanResponse,
		"cisco_nxos":    ciscoNxosCleanResponse,
		"juniper_junos": juniperJunosCleanResponse,
	}
}

// GetCleanFunc is only used for testing -- it returns a function that "cleans" output (usually a
// "show run" type of output) of any data that may change over time -- things like timestamps and
// password hashes, this allows for comparing the stored "golden" test data against "new" output
// gleaned from test clab devices.
func GetCleanFunc(platform string) func(r string) string {
	cleanFuncs := cleanResponseMap()

	cleanFunc, ok := cleanFuncs[platform]
	if !ok {
		return cleanResponseNoop
	}

	return cleanFunc
}

func replaceDoubleNewlines(s string) string {
	return strings.ReplaceAll(s, "\n\n", "\n")
}

func cleanResponseNoop(r string) string { return r }

type aristaEosReplacePatterns struct {
	datetimePattern        *regexp.Regexp
	datetimePatternNetconf *regexp.Regexp
	cryptoPattern          *regexp.Regexp
}

var aristaEosReplacePatternsInstance *aristaEosReplacePatterns //nolint:gochecknoglobals

func getAristaEosReplacePatterns() *aristaEosReplacePatterns {
	if aristaEosReplacePatternsInstance == nil {
		aristaEosReplacePatternsInstance = &aristaEosReplacePatterns{
			datetimePattern: regexp.MustCompile(
				`(?im)(mon|tue|wed|thu|fri|sat|sun)` +
					`\s+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)` +
					`\s+\d+\s+\d+:\d+:\d+\s+\d+$`,
			),
			datetimePatternNetconf: regexp.MustCompile(
				`\d+-\d+-\d+T\d+:\d+:\d+\.\d+Z`,
			),
			cryptoPattern: regexp.MustCompile(`(?im)secret\ssha512\s[\w$./]+$`),
		}
	}

	return aristaEosReplacePatternsInstance
}

func aristaEosCleanResponse(r string) string {
	replacePatterns := getAristaEosReplacePatterns()

	r = replacePatterns.datetimePattern.ReplaceAllString(r, "")
	r = replacePatterns.datetimePatternNetconf.ReplaceAllString(r, "")
	r = replacePatterns.cryptoPattern.ReplaceAllString(r, "")

	return replaceDoubleNewlines(r)
}

type ciscoIosxrReplacePatterns struct {
	datetimePattern         *regexp.Regexp
	cryptoPattern           *regexp.Regexp
	cfgByPattern            *regexp.Regexp
	commitInProgressPattern *regexp.Regexp
	passwordNetconfPattern  *regexp.Regexp
}

var ciscoIosxrReplacePatternsInstance *ciscoIosxrReplacePatterns //nolint:gochecknoglobals

func getCiscoIosxrReplacePatterns() *ciscoIosxrReplacePatterns {
	if ciscoIosxrReplacePatternsInstance == nil {
		ciscoIosxrReplacePatternsInstance = &ciscoIosxrReplacePatterns{
			datetimePattern: regexp.MustCompile(
				`(?im)(mon|tue|wed|thu|fri|sat|sun)` +
					`\s+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)` +
					`\s+\d+\s+\d+:\d+:\d+((\.\d+\s\w+)|\s\d+)`,
			),
			cryptoPattern: regexp.MustCompile(
				`(?im)(^\ssecret\s5\s[\w$./]+$)|(^\spassword\s7\s\w+$)`,
			),
			cfgByPattern: regexp.MustCompile(
				`(?im)^!! Last configuration change at TIME_STAMP_REPLACED by (\w+)$`,
			),
			commitInProgressPattern: regexp.MustCompile(`(?ims)System configuration.*`),
			passwordNetconfPattern:  regexp.MustCompile(`(?im)<password>.*</password>`),
		}
	}

	return ciscoIosxrReplacePatternsInstance
}

func ciscoIosxrCleanResponse(r string) string {
	replacePatterns := getCiscoIosxrReplacePatterns()

	r = replacePatterns.datetimePattern.ReplaceAllString(r, "")
	r = replacePatterns.cryptoPattern.ReplaceAllString(r, "")
	r = replacePatterns.cfgByPattern.ReplaceAllString(r, "")
	r = replacePatterns.commitInProgressPattern.ReplaceAllString(r, "")
	r = replacePatterns.passwordNetconfPattern.ReplaceAllString(r, "")

	return replaceDoubleNewlines(r)
}

type ciscoIosxeReplacePatterns struct {
	configBytesPattern *regexp.Regexp
	datetimePattern    *regexp.Regexp
	cryptoPattern      *regexp.Regexp
	cfgByPattern       *regexp.Regexp
	callHomePattern    *regexp.Regexp
	certLicensePattern *regexp.Regexp
	serialNetconf      *regexp.Regexp
	macAddrNetconf     *regexp.Regexp
	cryptoNetconf      *regexp.Regexp
}

var ciscoIosxeReplacePatternsInstance *ciscoIosxeReplacePatterns //nolint:gochecknoglobals

func getCiscoIosxeReplacePatterns() *ciscoIosxeReplacePatterns {
	if ciscoIosxeReplacePatternsInstance == nil {
		ciscoIosxeReplacePatternsInstance = &ciscoIosxeReplacePatterns{
			configBytesPattern: regexp.MustCompile(`(?im)^Current configuration : \d+ bytes$`),
			datetimePattern: regexp.MustCompile(
				`(?im)\d+:\d+:\d+\d+\s+[a-z]{3}\s+(mon|tue|wed|thu|fri|sat|sun)` +
					`\s+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d+\s+\d+`,
			),
			cryptoPattern: regexp.MustCompile(`(?im)^enable secret 9 (.*$)`),
			cfgByPattern: regexp.MustCompile(
				`(?im)^! Last configuration change at ([\w\s]+)?$`,
			),
			callHomePattern: regexp.MustCompile(
				`(?im)^! Call-home is enabled by Smart-Licensing.$`,
			),
			certLicensePattern: regexp.MustCompile(
				`(?ims)^crypto pki .*\nlicense udi pid CSR1000V sn \w+$`,
			),
			serialNetconf:  regexp.MustCompile(`(?i)<sn>\w+</sn>`),
			macAddrNetconf: regexp.MustCompile(`(?i)<mac-address>.*</mac-address>`),
			cryptoNetconf:  regexp.MustCompile(`(?i)<secret>.*</secret>`),
		}
	}

	return ciscoIosxeReplacePatternsInstance
}

func ciscoIosxeCleanResponse(r string) string {
	replacePatterns := getCiscoIosxeReplacePatterns()

	r = replacePatterns.configBytesPattern.ReplaceAllString(r, "")
	r = replacePatterns.datetimePattern.ReplaceAllString(r, "")
	r = replacePatterns.cryptoPattern.ReplaceAllString(r, "")
	r = replacePatterns.cfgByPattern.ReplaceAllString(r, "")
	r = replacePatterns.callHomePattern.ReplaceAllString(r, "")
	r = replacePatterns.certLicensePattern.ReplaceAllString(r, "")
	r = replacePatterns.serialNetconf.ReplaceAllString(r, "")
	r = replacePatterns.macAddrNetconf.ReplaceAllString(r, "")

	return replaceDoubleNewlines(r)
}

type ciscoNxosReplacePatterns struct {
	datetimePattern *regexp.Regexp
	cryptoPattern   *regexp.Regexp
	resourcePattern *regexp.Regexp
	datetimeNetconf *regexp.Regexp
}

var ciscoNxosReplacePatternsInstance *ciscoNxosReplacePatterns //nolint:gochecknoglobals

func getCiscoNxosReplacePatterns() *ciscoNxosReplacePatterns {
	if ciscoNxosReplacePatternsInstance == nil {
		ciscoNxosReplacePatternsInstance = &ciscoNxosReplacePatterns{
			datetimePattern: regexp.MustCompile(
				`(?im)(mon|tue|wed|thu|fri|sat|sun)\s+` +
					`(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d+\s+\d+:\d+:\d+\s\d+`,
			),
			cryptoPattern: regexp.MustCompile(`(?im)^(.*?\s(?:5|md5)\s)[\w$./]+.*$`),
			resourcePattern: regexp.MustCompile(
				`(?im)\d+\smaximum\s\d+$`,
			),
			datetimeNetconf: regexp.MustCompile(`<timeStamp>.*</timeStamp>`),
		}
	}

	return ciscoNxosReplacePatternsInstance
}

func ciscoNxosCleanResponse(r string) string {
	replacePatterns := getCiscoNxosReplacePatterns()

	r = replacePatterns.datetimePattern.ReplaceAllString(r, "")
	r = replacePatterns.cryptoPattern.ReplaceAllString(r, "")
	r = replacePatterns.resourcePattern.ReplaceAllString(r, "")

	return replaceDoubleNewlines(r)
}

type juniperJunosReplacePatterns struct {
	datetimePattern          *regexp.Regexp
	cryptoPattern            *regexp.Regexp
	commitSecNetconfPattern  *regexp.Regexp
	cryptoNetconfPattern     *regexp.Regexp
	datetimeNetconfPattern   *regexp.Regexp
	commitUserNetconfPattern *regexp.Regexp
}

var juniperJunosReplacePatternsInstance *juniperJunosReplacePatterns //nolint:gochecknoglobals

func getJuniperJunosReplacePatterns() *juniperJunosReplacePatterns {
	if juniperJunosReplacePatternsInstance == nil {
		juniperJunosReplacePatternsInstance = &juniperJunosReplacePatterns{
			datetimePattern: regexp.MustCompile(
				`(?im)^## Last commit: \d+-\d+-\d+\s\d+:\d+:\d+\s\w+.*$`,
			),
			cryptoPattern: regexp.MustCompile(
				`(?im)^\s+encrypted-password\s"[\w$./]+";\s.*$`,
			),
			commitSecNetconfPattern: regexp.MustCompile(`seconds="\d+"`),
			cryptoNetconfPattern: regexp.MustCompile(
				`<encrypted-password>.*</encrypted-password>`,
			),
			datetimeNetconfPattern: regexp.MustCompile(
				`localtime="\d+-\d+-\d+\s\d+:\d+:\d+\s\w+"`,
			),
			commitUserNetconfPattern: regexp.MustCompile(
				`commit-user="\w+"`,
			),
		}
	}

	return juniperJunosReplacePatternsInstance
}

func juniperJunosCleanResponse(r string) string {
	replacePatterns := getJuniperJunosReplacePatterns()

	r = replacePatterns.datetimePattern.ReplaceAllString(r, "")
	r = replacePatterns.cryptoPattern.ReplaceAllString(r, "")
	r = replacePatterns.commitSecNetconfPattern.ReplaceAllString(r, "")
	r = replacePatterns.cryptoNetconfPattern.ReplaceAllString(r, "")
	r = replacePatterns.datetimeNetconfPattern.ReplaceAllString(r, "")
	r = replacePatterns.commitUserNetconfPattern.ReplaceAllString(r, "")

	return replaceDoubleNewlines(r)
}
