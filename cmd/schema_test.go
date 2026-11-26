package cmd

import (
	"bytes"
	"testing"

	clabschemas "github.com/srl-labs/containerlab/schemas"
)

func TestSchemaCmd(t *testing.T) {
	c, err := schemaCmd(&Options{})
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{})

	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out.Bytes(), clabschemas.Topology()) {
		t.Fatal("schema command doesn't print the topology schema")
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
