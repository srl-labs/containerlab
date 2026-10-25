package sros

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

func TestModelDrivenAdminPassword(t *testing.T) {
	data := &srosTemplateData{
		Name:          "sr1",
		AdminUser:     `"admin"`,
		AdminPassword: `"Secret\"1!"`,
		IsSecureGrpc:  true,
	}

	modelDriven := ConfigVariant{Mode: ConfigModeModelDriven, Family: ConfigFamilySR}
	data.SwVersion = &SrosVersion{"26", "10", "R1"}
	out := executeVariantTemplate(t, modelDriven, data)
	require.Contains(t, out, `user "admin" password "Secret\"1!"`)
	require.Contains(t, out, `user "admin" access grpc true`)
	require.Contains(t, out, `user "admin" access netconf true`)
	require.NotContains(t, out, "md-cli admin-state enable")
	require.NotContains(t, out, "tls-aes128-ccm8-sha256")

	data.SwVersion = &SrosVersion{"26", "3", "R1"}
	out = executeVariantTemplate(t, modelDriven, data)
	require.Contains(t, out, "tls-aes128-ccm8-sha256")
	require.NotContains(t, out, "md-cli admin-state enable")

	data.IsSecureGrpc = false
	data.SwVersion = &SrosVersion{"0", "0", "0"}
	out = executeVariantTemplate(t, modelDriven, data)
	require.Contains(t, out, `password "Secret\"1!"`)
	require.NotContains(t, out, "tls-aes128-ccm8-sha256")

	data.IsSecureGrpc = true
	data.SrosLe25_10 = true
	data.AdminUser = `"operator"`
	data.SwVersion = &SrosVersion{"25", "10", "R1"}
	out = executeVariantTemplate(t, modelDriven, data)
	require.Contains(t, out, "md-cli admin-state enable")
	require.Contains(t, out, "md-cli-session permit")
	require.Contains(t, out, "tls-aes128-ccm8-sha256")
	require.Contains(t, out, `user "operator" access grpc true`)
	require.Contains(t, out, `user "operator" access netconf true`)
	require.NotContains(t, out, `user "admin" access`)
}

func TestSrosLe25_10(t *testing.T) {
	tests := []struct {
		name    string
		version *SrosVersion
		want    bool
	}{
		{name: "nil_is_latest", version: nil, want: false},
		{name: "development_is_latest", version: &SrosVersion{"0", "0", "0"}, want: false},
		{name: "25_10", version: &SrosVersion{"25", "10", "R1"}, want: true},
		{name: "26_3", version: &SrosVersion{"26", "3", "R1"}, want: false},
		{name: "26_10", version: &SrosVersion{"26", "10", "R1"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, srosLe25_10(tt.version))
		})
	}
}

func TestSrosLe26_3(t *testing.T) {
	tests := []struct {
		name    string
		version *SrosVersion
		want    bool
	}{
		{name: "nil_is_latest", version: nil, want: false},
		{name: "development_is_latest", version: &SrosVersion{"0", "0", "0"}, want: false},
		{name: "25_10", version: &SrosVersion{"25", "10", "R1"}, want: true},
		{name: "26_3", version: &SrosVersion{"26", "3", "R1"}, want: true},
		{name: "26_7", version: &SrosVersion{"26", "7", "R1"}, want: false},
		{name: "26_10", version: &SrosVersion{"26", "10", "R1"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, srosLe26_3(tt.version))
		})
	}
}

func TestClassicTemplateAdminPassword(t *testing.T) {
	tpl, err := template.New("classic").Funcs(srosTemplateFuncs(nil)).Parse(cfgTplClassic)
	require.NoError(t, err)

	data := &srosTemplateData{
		Name:              "sr1",
		AdminUser:         `"admin"`,
		AdminPassword:     `"Secret1!"`,
		ConfigurationMode: string(ConfigModeClassic),
		IFaces:            map[string]tplIFace{},
	}

	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, data))
	out := buf.String()
	require.Contains(t, out, `user "admin"`)
	require.Contains(t, out, `password "Secret1!"`)
	require.NotContains(t, out, "NokiaSros1!")
}

func TestClassicTemplateCcm8(t *testing.T) {
	templates := map[string]string{
		"sr":  cfgTplClassic,
		"ixr": cfgTplClassicIxr,
		"sar": cfgTplClassicSar,
	}
	for name, source := range templates {
		t.Run(name, func(t *testing.T) {
			data := &srosTemplateData{
				AdminUser:         `"admin"`,
				AdminPassword:     `"Secret1!"`,
				ConfigurationMode: string(ConfigModeClassic),
				IsSecureGrpc:      true,
				SwVersion:         &SrosVersion{"26", "7", "R1"},
				IFaces:            map[string]tplIFace{},
			}
			out := executeClassicTemplate(t, name, source, data)
			require.NotContains(t, out, "tls-aes128-ccm8-sha256")

			data.SwVersion = &SrosVersion{"26", "3", "R1"}
			out = executeClassicTemplate(t, name, source, data)
			require.Contains(t, out, "tls-aes128-ccm8-sha256")
		})
	}
}

func executeClassicTemplate(t *testing.T, name, source string, data *srosTemplateData) string {
	t.Helper()
	tpl, err := template.New(name).Funcs(srosTemplateFuncs(data.SwVersion)).Parse(source)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, data))
	return buf.String()
}

func executeVariantTemplate(
	t *testing.T, v ConfigVariant, data *srosTemplateData,
) string {
	t.Helper()
	tmpl, name := getTemplateForVariant(v, data.SwVersion)
	tpl, err := template.New(name).Funcs(srosTemplateFuncs(data.SwVersion)).Parse(tmpl)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, data))
	return buf.String()
}
