package gotextfsm

import (
	"regexp"
	"sort"
	"strings"
	"text/template"
)

// ExecutePythonTemplate converts a python template into a golang template and executes
// against the given variable map.
// This is a quick hack and not does not support all comprehensive list of Python Template package.
// It does the following:
// 	* replace all ${varname} to {{.varname}}
// 	* replace all $varname to {{.varname}} (By sorting the varnames with the longest name first)
// 	* replace all $$ with $
//  * escape all literal {{ and }} with {{"{{"}} and {{"}}"}}
// Assumes it is a valid python Template. No validations done to validate the python template syntax.
// Assumes all the variables are proper variable identifiers
// Then executes the resulting golang template on the map passed.
func ExecutePythonTemplate(pytemplate string, vars_map map[string]interface{}) (string, error) {
	t := pytemplate
	// Replace ${xxxx} with {{.xxxx}}
	r1 := regexp.MustCompile(`\$\{([^$\{\}]+)\}`)
	t = r1.ReplaceAllString(t, "__DOUBLE_OPENBR__ .$1 __DOUBLE_CLOSEBR__")
	if vars_map != nil {
		keys := make([]string, 0, len(vars_map))
		for k := range vars_map {
			keys = append(keys, k)
		}
		// Sort by longest key first.
		// This is done so that $var1234 is replaced first before replacing $var12 for example.
		sort.SliceStable(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
		for _, key := range keys {
			t = regexp.MustCompile(`\$(`+key+`)`).ReplaceAllString(t, "__DOUBLE_OPENBR__ .$1 __DOUBLE_CLOSEBR__")
		}
	}
	// Replace $$ with $
	t = strings.ReplaceAll(t, "$$", "$")
	// Escape { and } with {{"{"}} and {{"}"}}
	// Generally speaking, golang template has special meeaning for {{ and }}. Hence, we should escape only {{ and }}
	//
	// But. Looks like golang template barks at something like \{{{.INBOUND_SETTINGS_IN_USE}}
	// Hence, we escape every single { and } instead of only {{ and }}
	t = strings.ReplaceAll(t, "{", `__DOUBLE_OPENBR__"{"__DOUBLE_CLOSEBR__`)
	t = strings.ReplaceAll(t, "}", `__DOUBLE_OPENBR__"}"__DOUBLE_CLOSEBR__`)
	t = strings.ReplaceAll(t, "__DOUBLE_OPENBR__", `{{`)
	t = strings.ReplaceAll(t, "__DOUBLE_CLOSEBR__", `}}`)
	var sb strings.Builder
	gotemplate, err := template.New("test").Parse(t)
	if err != nil {
		return "", err
	}
	gotemplate.Execute(&sb, vars_map)
	return sb.String(), nil
}
