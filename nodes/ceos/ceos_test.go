// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ceos

import "testing"

func TestCeosDoesNotSupportLiveLinkApply(t *testing.T) {
	if (&ceos{}).SupportsLiveLinkApply() {
		t.Fatal("expected cEOS to require restart on apply link changes")
	}
}
