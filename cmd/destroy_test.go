package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDestroyKeepLinksRequiresNodeFilter(t *testing.T) {
	o := GetOptions()
	copyOptions := *o
	destroyOptions := *o.Destroy
	filterOptions := *o.Filter
	copyOptions.Destroy = &destroyOptions
	copyOptions.Filter = &filterOptions
	copyOptions.Destroy.KeepLinks = true
	copyOptions.Filter.NodeFilter = nil

	err := destroyFn(&cobra.Command{}, &copyOptions)
	if err == nil || !strings.Contains(err.Error(), "keep-links requires node-filter") {
		t.Fatalf("destroyFn() error = %v, want keep-links/node-filter validation error", err)
	}
}

func TestDestroyKeepLinksRejectsCleanup(t *testing.T) {
	o := GetOptions()
	copyOptions := *o
	destroyOptions := *o.Destroy
	filterOptions := *o.Filter
	copyOptions.Destroy = &destroyOptions
	copyOptions.Filter = &filterOptions
	copyOptions.Destroy.KeepLinks = true
	copyOptions.Destroy.Cleanup = true
	copyOptions.Filter.NodeFilter = []string{"node1"}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := destroyFn(cmd, &copyOptions)
	if err == nil || !strings.Contains(err.Error(), "keep-links cannot be used with cleanup") {
		t.Fatalf("destroyFn() error = %v, want keep-links/cleanup validation error", err)
	}
}
