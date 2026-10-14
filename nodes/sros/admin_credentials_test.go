package sros

import (
	"bytes"
	"testing"
	"text/template"

	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/stretchr/testify/require"
)

func TestApplyAdminCredentials(t *testing.T) {
	in := `/configure system security user-params local-user user __CLAB_ADMIN_USER__ password __CLAB_ADMIN_PASSWORD__`

	got, err := applyAdminCredentials(in, "admin", "NokiaSros1!", false)
	require.NoError(t, err)
	require.Equal(t,
		`/configure system security user-params local-user user "admin" password "NokiaSros1!"`,
		got,
	)
	require.NotContains(t, got, "new-password-at-login")

	got, err = applyAdminCredentials(in, "admin", `Secret"1!`, true)
	require.NoError(t, err)
	require.Contains(t, got, `password "Secret\"1!"`)
	require.Contains(t, got, "password minimum-age 0")
	require.Contains(t, got, `user "admin" console new-password-at-login true`)

	_, err = applyAdminCredentials("no placeholders", "admin", "NokiaSros1!", false)
	require.Error(t, err)
}

func TestClassicTemplateAdminPassword(t *testing.T) {
	tpl, err := template.New("classic").Funcs(clabutils.CreateFuncs()).Parse(cfgTplClassic)
	require.NoError(t, err)

	data := &srosTemplateData{
		Name:              "sr1",
		AdminUser:         `"admin"`,
		AdminPassword:     `"Secret1!"`,
		PromptPassword:    true,
		ConfigurationMode: string(ConfigModeClassic),
		IFaces:            map[string]tplIFace{},
	}

	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, data))
	out := buf.String()
	require.Contains(t, out, `user "admin"`)
	require.Contains(t, out, `password "Secret1!"`)
	require.Contains(t, out, "minimum-age sec 0")
	require.Contains(t, out, "new-password-at-login")
	require.NotContains(t, out, "NokiaSros1!")

	data.PromptPassword = false
	buf.Reset()
	require.NoError(t, tpl.Execute(&buf, data))
	require.Contains(t, buf.String(), `password "Secret1!"`)
	require.NotContains(t, buf.String(), "new-password-at-login")
}
