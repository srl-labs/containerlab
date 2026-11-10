// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package theme

import (
	"strings"
	"testing"
)

func TestEveryNamedThemeBuilds(t *testing.T) {
	names := Names()
	if len(names) < 3 || names[0] != Auto {
		t.Fatalf("unexpected theme names: %v", names)
	}

	seen := map[string]bool{}

	for _, name := range names {
		if seen[name] {
			t.Errorf("theme %q is listed twice", name)
		}

		seen[name] = true

		th, err := New(name, true)
		if err != nil {
			t.Errorf("theme %q does not build: %v", name, err)

			continue
		}

		// Every theme needs the colors the UI cannot do without.
		p := th.Palette
		for label, c := range map[string]any{
			"muted": p.Muted, "accent": p.Accent, "on accent": p.OnAccent,
			"selection": p.Selection, "border": p.Border, "raised": p.Raised,
			"error": p.Error, "warning": p.Warning, "info": p.Info, "success": p.Success,
		} {
			if c == nil {
				t.Errorf("theme %q has no %s color", name, label)
			}
		}

		if len(p.Hues) == 0 {
			t.Errorf("theme %q has no hues to tell kinds apart", name)
		}
	}
}

func TestAutoThemeFollowsTheTerminal(t *testing.T) {
	dark, _ := New(Auto, true)
	light, _ := New("", false)

	if !dark.Dark || light.Dark {
		t.Errorf("auto resolved to dark=%v for a dark and dark=%v for a light terminal",
			dark.Dark, light.Dark)
	}

	// The automatic themes keep the terminal's own background, the named ones paint theirs.
	if dark.Opaque() || light.Opaque() {
		t.Error("the automatic themes should not paint the background")
	}

	if named, _ := New("nord", false); !named.Opaque() || !named.Dark {
		t.Error("a named theme should paint its background, whatever the terminal")
	}

	if !IsAuto("") || !IsAuto(" Auto ") || IsAuto("dark") {
		t.Error("IsAuto does not tell the automatic theme apart")
	}
}

func TestUnknownTheme(t *testing.T) {
	_, err := New("no-such-theme", true)
	if err == nil || !strings.Contains(err.Error(), "available themes") {
		t.Errorf("expected an error listing the themes, got %v", err)
	}
}

func TestHueIsStablePerFamily(t *testing.T) {
	th := Default(true)

	if th.Hue("nokia_srlinux") != th.Hue("nokia_srsim") {
		t.Error("kinds of one vendor should share a color")
	}

	if th.Hue("nokia_srlinux") == th.Hue("linux") {
		t.Error("the two most common families should be told apart")
	}

	if th.Hue("some_unknown_kind") != th.Hue("some_other") {
		t.Error("an unknown family should still map to one color")
	}

	if th.Hue("") == nil {
		t.Error("an empty name should still get a color")
	}
}
