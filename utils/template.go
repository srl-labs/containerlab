package utils

import (
	"encoding/json"
	"text/template"
)

var TemplateFuncs = template.FuncMap{
	"ToJSON":       toJson,
	"ToJSONPretty": toJsonPretty,
	"add":          add,
}

func toJson(v any) string {
	a, _ := json.Marshal(v)

	return string(a)
}

func toJsonPretty(v any, prefix string, indent string) string {
	a, _ := json.MarshalIndent(v, prefix, indent)
	return string(a)
}

func add(a, b int) int {
	return a + b
}
