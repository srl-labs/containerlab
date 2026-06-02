// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type flagInput struct {
	kind string
	lics []string
}
type flagOutput struct {
	out map[string]string
	err error
}

type nodesInput struct {
	kind  string
	nodes []string
}
type nodesOutput struct {
	out []nodesDef
	err error
}

var flagTestSet = map[string]struct {
	in  flagInput
	out flagOutput
}{
	"no_kind": {
		in: flagInput{
			kind: "srl",
			lics: []string{"/path/to/license.key"},
		},
		out: flagOutput{
			out: map[string]string{"srl": "/path/to/license.key"},
			err: nil,
		},
	},
	"1_item_without_kind": {
		in: flagInput{
			kind: "srl",
			lics: []string{"/path1"},
		},
		out: flagOutput{
			out: map[string]string{"srl": "/path1"},
			err: nil,
		},
	},
	"1_item_with_kind": {
		in: flagInput{
			kind: "srl",
			lics: []string{"ceos=/path/to/license.key"},
		},
		out: flagOutput{
			out: map[string]string{"ceos": "/path/to/license.key"},
			err: nil,
		},
	},
	"2_items_with_kind": {
		in: flagInput{
			kind: "dummy",
			lics: []string{"srl=/path1", "ceos=/path2"},
		},
		out: flagOutput{
			out: map[string]string{"srl": "/path1", "ceos": "/path2"},
			err: nil,
		},
	},
	"2_items_without_kind": {
		in: flagInput{
			kind: "srl",
			lics: []string{"/path1", "/path2"},
		},
		out: flagOutput{
			out: nil,
			err: errDuplicatedValue,
		},
	},
	"1_item_without_kind_1_item_with_kind": {
		in: flagInput{
			kind: "srl",
			lics: []string{"/path1", "ceos=/path2"},
		},
		out: flagOutput{
			out: map[string]string{"srl": "/path1", "ceos": "/path2"},
			err: nil,
		},
	},
}

var nodesTestSet = map[string]struct {
	in  nodesInput
	out nodesOutput
}{
	"no_kind_no_nodes": {
		in:  nodesInput{kind: "", nodes: nil},
		out: nodesOutput{out: nil, err: errSyntax},
	},
	"no_kind_with_nodes_1": {
		in: nodesInput{
			kind:  "",
			nodes: []string{"1", "2", "3"},
		},
		out: nodesOutput{out: nil, err: errSyntax},
	},
	"no_kind_with_nodes_2": {
		in: nodesInput{
			kind:  "",
			nodes: []string{"1:srl", "2", "3"},
		},
		out: nodesOutput{out: nil, err: errSyntax},
	},
	"no_kind_with_nodes_3": {
		in: nodesInput{
			kind:  "",
			nodes: []string{"1:srl", "2:ceos", "3"},
		},
		out: nodesOutput{out: nil, err: errSyntax},
	},
	"kind_nodes_only_uints": {
		in: nodesInput{
			kind:  "srl",
			nodes: []string{"1", "2", "3"},
		},
		out: nodesOutput{
			out: []nodesDef{
				{numNodes: 1, kind: "srl", typ: ""},
				{numNodes: 2, kind: "srl", typ: ""},
				{numNodes: 3, kind: "srl", typ: ""},
			},
			err: nil,
		},
	},
	"kind_nodes_with_kind": {
		in: nodesInput{
			kind:  "srl",
			nodes: []string{"1:linux", "2", "3:ceos"},
		},
		out: nodesOutput{
			out: []nodesDef{
				{numNodes: 1, kind: "linux", typ: ""},
				{numNodes: 2, kind: "srl", typ: ""},
				{numNodes: 3, kind: "ceos", typ: ""},
			},
			err: nil,
		},
	},
	"kind_nodes_with_kind_and_type": {
		in: nodesInput{
			kind:  "srl",
			nodes: []string{"1::ixrd", "2", "3:ceos"},
		},
		out: nodesOutput{
			out: []nodesDef{
				{numNodes: 1, kind: "srl", typ: "ixrd"},
				{numNodes: 2, kind: "srl"},
				{numNodes: 3, kind: "ceos", typ: ""},
			},
			err: nil,
		},
	},
	"single_stage": {
		in: nodesInput{
			kind:  "srl",
			nodes: []string{"2"},
		},
		out: nodesOutput{
			out: []nodesDef{
				{numNodes: 2, kind: "srl", typ: ""},
			},
			err: nil,
		},
	},
}

func TestParseFlag(t *testing.T) {
	for name, set := range flagTestSet {
		t.Run(name, func(t *testing.T) {
			result, err := parseFlag(set.in.kind, set.in.lics)
			if !cmp.Equal(result, set.out.out) {
				t.Errorf("failed at '%s', expected %v, got %+v", name, set.out.out, result)
			}

			if err != set.out.err {
				t.Errorf("failed at '%s', expected error %+v, got %+v", name, set.out.err, err)
			}
		})
	}
}

func TestParseNodes(t *testing.T) {
	for name, set := range nodesTestSet {
		t.Run(name, func(t *testing.T) {
			result, err := parseNodesFlag(set.in.kind, set.in.nodes...)
			if !cmp.Equal(result, set.out.out, cmp.AllowUnexported(nodesDef{})) {
				t.Errorf("failed at '%s', expected %+v, got %+v", name, set.out.out, result)
			}

			if err != set.out.err {
				t.Errorf("failed at '%s', expected error %+v, got %+v", name, set.out.err, err)
			}
		})
	}
}
