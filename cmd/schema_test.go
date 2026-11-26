package cmd

import (
	"bytes"
	"testing"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabschemas "github.com/srl-labs/containerlab/schemas"
)

func TestSchemaCmd(t *testing.T) {
	t.Setenv("CLAB_VERSION_CHECK", "disable")

	// podman requires root for the other commands, schema doesn't use the runtime
	for _, runtime := range []string{"", "docker", "podman", clablabruntime.ClabernetesRuntimeName} {
		t.Run("runtime "+runtime, func(t *testing.T) {
			optionsInstance = nil
			t.Cleanup(func() { optionsInstance = nil })

			root, err := Entrypoint()
			if err != nil {
				t.Fatal(err)
			}

			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs([]string{"--runtime", runtime, "schema"})

			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(out.Bytes(), clabschemas.Topology()) {
				t.Fatal("schema command doesn't print the topology schema")
			}
		})
	}
}

func TestSchemaCmdRejectsArguments(t *testing.T) {
	c, err := schemaCmd(&Options{})
	if err != nil {
		t.Fatal(err)
	}

	c.SetOut(&bytes.Buffer{})
	c.SetErr(&bytes.Buffer{})
	c.SetArgs([]string{"topology"})

	if err := c.Execute(); err == nil {
		t.Fatal("schema command accepted an argument")
	}
}
