// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

var update = flag.Bool("update", false, "rewrite the golden files with the current output")

// ansiDirEnv names a directory the screens are also written to with their colors, for
// turning them into images.
const ansiDirEnv = "CLAB_TUI_ANSI_DIR"

func fixedClock() time.Time {
	return time.Date(2026, time.November, 10, 12, 4, 11, 0, time.UTC)
}

// newTestModel returns a UI of the given size on top of the demo backend.
func newTestModel(t *testing.T, width, height int) (*Model, *clabinternaltuibackendfake.Backend) {
	t.Helper()

	be := clabinternaltuibackendfake.New(
		clabinternaltuibackendfake.Options{Now: fixedClock, Seed: 1},
	)
	m := New(be, Options{Now: fixedClock, Home: "/home/demo"})
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	selectLab(t, m, "dc1")

	return m, be
}

// selectLab moves the cursor of the lab list to the lab with the given name. The tests
// name the lab they work on, so that the order of the list does not matter to them.
func selectLab(t *testing.T, m *Model, name string) {
	t.Helper()

	page, ok := m.top().(*labsPage)
	if !ok {
		t.Fatalf("expected the lab list, got %T", m.top())
	}

	for i := range m.labs {
		if m.labs[i].Name == name && page.tbl.SetCursorKey(labKey(&m.labs[i])) {
			return
		}
	}

	t.Fatalf("lab %q is not in the list", name)
}

// keyMsg builds the key press for a key given by name, ie. "enter" or "D".
func keyMsg(name string) tea.KeyPressMsg {
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "home": tea.KeyHome, "end": tea.KeyEnd,
		"backspace": tea.KeyBackspace,
	}

	if code, ok := named[name]; ok {
		return tea.KeyPressMsg{Code: code}
	}

	if name == "space" {
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}

	if rest, ok := strings.CutPrefix(name, "ctrl+"); ok {
		code, _ := utf8.DecodeRuneInString(rest)

		return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
	}

	r, _ := utf8.DecodeRuneInString(name)
	msg := tea.KeyPressMsg{Code: unicode.ToLower(r), Text: name}

	if unicode.IsUpper(r) {
		msg.Mod = tea.ModShift
	}

	return msg
}

// press sends key presses to the UI, running the commands they return is left out: the
// tests look at the state right after the keys.
func press(m *Model, keys ...string) {
	for _, k := range keys {
		m.Update(keyMsg(k))
	}
}

// run executes a command the way the program would and feeds its message back into the
// UI, following the commands that come out of it. Commands that do not return promptly,
// such as timers, are left alone.
func run(m *Model, cmd tea.Cmd, depth int) {
	const maxDepth = 6

	if cmd == nil || depth > maxDepth {
		return
	}

	done := make(chan tea.Msg, 1)

	go func() { done <- cmd() }()

	select {
	case msg := <-done:
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				run(m, c, depth+1)
			}

			return
		}

		_, next := m.Update(msg)
		run(m, next, depth+1)
	case <-time.After(150 * time.Millisecond):
	}
}

// pressRun sends a key press and runs the command it returns.
func pressRun(m *Model, key string) {
	_, cmd := m.Update(keyMsg(key))
	run(m, cmd, 0)
}

// advance moves the simulation on by a number of steps and lets the UI pick the changes up.
func advance(m *Model, be *clabinternaltuibackendfake.Backend, steps int) {
	for range steps {
		be.Tick()
	}

	m.Update(flushMsg{})
}

// typeText sends a text one character at a time.
func typeText(m *Model, text string) {
	for _, r := range text {
		if r == ' ' {
			m.Update(keyMsg("space"))

			continue
		}

		m.Update(keyMsg(string(r)))
	}
}

// snapshot compares the screen with its golden file, without colors.
func snapshot(t *testing.T, m *Model, name string) {
	t.Helper()

	screen := m.render()
	assertScreenFits(t, m, screen)

	if dir := os.Getenv(ansiDirEnv); dir != "" {
		if err := os.WriteFile(
			filepath.Join(dir, name+".ansi"),
			[]byte(screen),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}

	lines := strings.Split(ansi.Strip(screen), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}

	got := strings.Join(lines, "\n") + "\n"
	path := filepath.Join("testdata", name+".txt")

	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file: %v (run the tests with -update to create it)", err)
	}

	if got != string(want) {
		t.Errorf("screen %s differs from %s:\n%s", name, path, got)
	}
}

// assertScreenFits checks that a screen is exactly as large as the window: no line may
// wrap or be missing, whatever the content.
func assertScreenFits(t *testing.T, m *Model, screen string) {
	t.Helper()

	lines := strings.Split(screen, "\n")
	if len(lines) != m.height {
		t.Errorf("screen has %d lines, the window %d", len(lines), m.height)
	}

	for i, line := range lines {
		if w := ansi.StringWidth(line); w != m.width {
			t.Errorf("line %d is %d cells wide, the window %d: %q", i, w, m.width, ansi.Strip(line))
		}
	}
}

func TestScreenLabs(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	snapshot(t, m, "labs_120x35")
}

func TestScreenNodes(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "down", "down", "down", "down", "space", "space")
	snapshot(t, m, "nodes_120x35")
}

func TestScreenNodesFiltered(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "2", "/")
	typeText(m, "leaf")
	snapshot(t, m, "nodes_filter_120x35")
}

func TestScreenNode(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "down", "down", "enter", "down", "down")
	snapshot(t, m, "node_120x35")
}

func TestScreenHelp(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "?")
	snapshot(t, m, "help_120x35")
}

func TestScreenNodesNarrow(t *testing.T) {
	m, _ := newTestModel(t, 80, 24)
	press(m, "enter")
	snapshot(t, m, "nodes_80x24")
}

// TestScreensFitAnySize renders the main screens at many window sizes: every line must
// be exactly as wide as the window, so that nothing ever wraps.
func TestScreensFitAnySize(t *testing.T) {
	steps := [][]string{nil, {"enter"}, {"enter", "enter"}, {"2"}, {"?"}}

	for width := minWidth; width <= 200; width += 7 {
		for _, height := range []int{minHeight, 20, 35, 60} {
			for _, keys := range steps {
				m, _ := newTestModel(t, width, height)
				press(m, keys...)
				assertScreenFits(t, m, m.render())
			}
		}
	}
}

func TestScreenInterfaces(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "3")
	snapshot(t, m, "interfaces_120x35")
}

func TestScreenLinks(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "4")
	snapshot(t, m, "links_120x35")
}

func TestScreenDestroyDialog(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "X", "up", "up", "space")
	snapshot(t, m, "destroy_dialog_120x35")
}

func TestScreenDeployJob(t *testing.T) {
	m, be := newTestModel(t, 120, 35)
	selectLab(t, m, "clos")
	press(m, "D", "enter")
	advance(m, be, 3)
	snapshot(t, m, "deploy_job_120x35")

	advance(m, be, 6)
	press(m, "6")
	snapshot(t, m, "jobs_120x35")
}

func TestScreenLogs(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "down", "down")
	pressRun(m, "l")
	press(m, "T", "/")
	typeText(m, "bgp")
	snapshot(t, m, "logs_120x35")
}

func TestScreenNetemDialog(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "down", "down", "enter", "down", "n")
	snapshot(t, m, "netem_dialog_120x35")
}

func TestScreenConnectMenu(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "c")
	typeText(m, "ba")
	snapshot(t, m, "connect_menu_120x35")
}

// The three screens below are what a newcomer leans on: the menu of everything that can be
// done, the finder that goes anywhere by name, and a menu answered with a single key.
func TestScreenActionMenu(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "a")
	snapshot(t, m, "action_menu_120x35")
}

func TestScreenGoTo(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "ctrl+k")
	typeText(m, "pe")
	snapshot(t, m, "goto_120x35")
}

func TestScreenCopyMenu(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "y")
	snapshot(t, m, "copy_menu_120x35")
}

func TestScreenRunCommand(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "ctrl+a", "x")
	typeText(m, "ip -br addr")
	pressRun(m, "enter")
	snapshot(t, m, "run_command_120x35")
}

func TestScreenDescribe(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter")
	pressRun(m, "d")
	snapshot(t, m, "describe_120x35")
}

// TestDestroyRemovesLab walks through destroying a lab: confirm, watch the job, and see
// the lab listed as not deployed afterwards.
func TestDestroyRemovesLab(t *testing.T) {
	m, be := newTestModel(t, 120, 35)
	press(m, "X")

	if len(m.overlays) != 1 {
		t.Fatalf("expected the confirmation dialog, got %d dialogs", len(m.overlays))
	}

	// Cancel is the default of a destructive dialog: enter must not destroy anything.
	press(m, "enter")

	if len(be.Jobs()) != 0 {
		t.Fatal("enter on the default button started the destroy")
	}

	press(m, "X", "right", "enter")
	advance(m, be, 10)

	if lab := m.lab("dc1"); lab == nil || lab.Deployed {
		t.Fatalf("dc1 should be listed as not deployed, got %+v", lab)
	}
}

// TestFilterKeepsTypedLettersOutOfNavigation types letters that are navigation keys of
// the table into the filter: they must end up in the query.
func TestFilterKeepsTypedLettersOutOfNavigation(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "2", "/")
	typeText(m, "jk")

	if got := m.top().table().Filter(); got != "jk" {
		t.Errorf("filter is %q, want %q", got, "jk")
	}

	press(m, "esc")

	if got := m.top().table().Filter(); got != "" {
		t.Errorf("escape left the filter %q behind", got)
	}
}

// TestThemesRender draws a screen in every theme. Themes only change colors: the text on
// the screen must be the same in all of them.
func TestThemesRender(t *testing.T) {
	var reference string

	for _, name := range clabinternaltuitheme.Names() {
		m, _ := newTestModel(t, 120, 35)
		press(m, "enter", "down", "down", "space")

		if cmd := m.runCommand("theme " + name); cmd != nil {
			t.Fatalf("theme %s: unexpected command", name)
		}

		if m.th.ID != name && name != clabinternaltuitheme.Auto {
			t.Fatalf("theme %s was not applied, the UI uses %s", name, m.th.ID)
		}

		screen := m.render()
		assertScreenFits(t, m, screen)

		if dir := os.Getenv(ansiDirEnv); dir != "" {
			path := filepath.Join(dir, "theme_"+name+".ansi")
			if err := os.WriteFile(path, []byte(screen), 0o600); err != nil {
				t.Fatal(err)
			}
		}

		text := ansi.Strip(screen)
		// The status line names the theme that was just selected: leave it out.
		text = text[:strings.LastIndex(text, "\n")]

		if reference == "" {
			reference = text
		} else if text != reference {
			t.Errorf("theme %s changes the text on the screen", name)
		}

		if view := m.View(); m.th.Opaque() != (view.BackgroundColor != nil) {
			t.Errorf("theme %s: the window background does not follow the theme", name)
		}
	}
}

func TestUnknownThemeIsRefused(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	before := m.th.ID

	m.runCommand("theme no-such-theme")

	if m.th.ID != before {
		t.Errorf("an unknown theme replaced %s with %s", before, m.th.ID)
	}

	if !strings.Contains(m.toast.text, "unknown theme") {
		t.Errorf("the user is not told about the unknown theme: %q", m.toast.text)
	}
}

// TestQuitAsksWhileJobsRun starts an operation and quits: the UI must ask first, as
// quitting cuts the operation short.
func TestQuitAsksWhileJobsRun(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)

	if cmd := m.quit(); cmd == nil {
		t.Fatal("quitting with nothing running should quit at once")
	}

	selectLab(t, m, "clos")
	press(m, "D", "enter")
	m.Update(flushMsg{})

	if len(m.jobs) != 1 || m.jobs[0].State.Done() {
		t.Fatalf("expected one running job, got %+v", m.jobs)
	}

	press(m, "ctrl+c")

	if len(m.overlays) != 1 {
		t.Fatalf("expected the question whether to quit, got %d dialogs", len(m.overlays))
	}

	// The default answer of the question is not to quit.
	_, cmd := m.Update(keyMsg("enter"))
	if cmd != nil || len(m.overlays) != 0 {
		t.Error("enter on the default button should close the question and stay")
	}

	// A second ctrl+c while the question is open quits without an answer.
	press(m, "ctrl+c")

	if _, cmd := m.Update(keyMsg("ctrl+c")); cmd == nil {
		t.Error("a second ctrl+c should quit")
	}
}

func TestDoubleClickOpensRow(t *testing.T) {
	now := fixedClock()
	be := clabinternaltuibackendfake.New(
		clabinternaltuibackendfake.Options{Now: fixedClock, Seed: 1},
	)
	m := New(be, Options{Now: func() time.Time { return now }, Home: "/home/demo"})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})

	// Line 3 of the screen is the first row of the lab list.
	click := tea.MouseClickMsg{X: 10, Y: 3, Button: tea.MouseLeft}

	m.Update(click)

	labs, isLabs := m.top().(*labsPage)
	if !isLabs {
		t.Fatal("a single click opened the row")
	}

	first := labs.current(m).Name

	now = now.Add(time.Second)

	m.Update(click)

	if _, isLabs := m.top().(*labsPage); !isLabs {
		t.Fatal("two slow clicks were taken for a double click")
	}

	now = now.Add(100 * time.Millisecond)

	m.Update(click)

	if _, isNodes := m.top().(*nodesPage); !isNodes || m.scope != first {
		t.Errorf("a double click did not open %s, the UI shows %T scoped to %q",
			first, m.top(), m.scope)
	}
}

func TestClickOnTabSwitchesView(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	m.render() // lays the tabs out

	var jobs zone

	for _, z := range m.tabZones {
		if z.tab == tabJobs {
			jobs = z
		}
	}

	m.Update(tea.MouseClickMsg{X: jobs.from + 1, Y: 0, Button: tea.MouseLeft})

	if _, ok := m.top().(*jobsPage); !ok {
		t.Errorf("a click on the jobs tab shows %T", m.top())
	}
}

func TestCommandLine(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)

	press(m, ":")
	typeText(m, "lab sp")
	press(m, "tab", "enter")

	if m.scope != "sp-core" {
		t.Errorf("tab should complete the lab name, the scope is %q", m.scope)
	}

	if _, ok := m.top().(*nodesPage); !ok {
		t.Errorf(":lab should show the nodes of the lab, got %T", m.top())
	}

	press(m, ":")
	typeText(m, "all")
	press(m, "enter")

	if m.scope != "" {
		t.Errorf(":all should show every lab again, the scope is %q", m.scope)
	}

	press(m, ":")
	typeText(m, "nonsense")
	press(m, "enter")

	if !strings.Contains(m.toast.text, "neither a command nor the name") {
		t.Errorf("an unknown command should be reported, got %q", m.toast.text)
	}
}

// TestActionsNeedTheirCapability runs the UI on a backend that cannot change anything and
// checks that the operations are refused with the reason.
func TestActionsNeedTheirCapability(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	m.host.Privileged = false
	m.host.Caps.Lifecycle = false
	m.host.Caps.Reasons = map[string]string{"lifecycle": "root privileges required"}

	press(m, "X")

	if len(m.overlays) != 0 {
		t.Error("destroy was offered although the backend cannot do it")
	}

	if !strings.Contains(m.toast.text, "root privileges required") {
		t.Errorf("the reason is not shown: %q", m.toast.text)
	}

	if !strings.Contains(ansi.Strip(m.render()), "read-only (no root)") {
		t.Error("the status line does not flag the read-only mode")
	}
}

func TestScreenMap(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "5", "down", "tab")
	snapshot(t, m, "map_120x35")
}

// TestMapLinkLabels walks through what the map writes on its links: the names of the
// interfaces, for the node in focus, for every link or not at all, and the traffic rates,
// which can be switched off as well. The choice is kept when the map moves on to another lab.
func TestMapLinkLabels(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "5", "down")

	page, ok := m.top().(*mapPage)
	if !ok || page.node != "leaf1" {
		t.Fatalf("expected the map with the focus on leaf1, got %T", m.top())
	}

	body := func() string { return ansi.Strip(m.render()) }

	// The uplinks of the node in focus are named, the links of other nodes are not.
	focus := body()
	if !strings.Contains(focus, "e1-49") || !strings.Contains(focus, "e1-50") {
		t.Fatalf("the links of the node in focus are not named:\n%s", focus)
	}

	names := strings.Count(focus, "e1-")

	press(m, "i")

	all := body()
	if got := strings.Count(all, "e1-"); got <= names {
		t.Errorf("naming every link wrote %d names, the focus alone had %d:\n%s", got, names, all)
	}

	snapshot(t, m, "map_names_all_120x35")

	press(m, "i")

	// The panel next to the map still lists the interfaces, the drawing does not.
	page.hidePanel = true
	page.resize(page.width, page.height)

	if off := body(); strings.Contains(off, "e1-") {
		t.Errorf("the names are still written after switching them off:\n%s", off)
	}

	if !strings.Contains(body(), "M") || page.labels.rates != true {
		t.Fatal("the traffic rates are gone with the names")
	}

	rates := regexp.MustCompile(`\d(\.\d)?[kMG]\b`)
	if !rates.MatchString(strings.Join(strings.Split(body(), "\n")[2:20], "\n")) {
		t.Fatalf("no traffic rate is written on the map:\n%s", body())
	}

	press(m, "r")

	if lines := strings.Split(body(), "\n")[2:20]; rates.MatchString(clearNotes(lines)) {
		t.Errorf("the rates are still written after switching them off:\n%s", body())
	}

	// The map of another lab opens with the same choice, and one more step brings the
	// names back.
	press(m, "1")
	selectLab(t, m, "campus")
	press(m, "enter", "5")

	other, ok := m.top().(*mapPage)
	if !ok || other.lab != "campus" {
		t.Fatalf("expected the map of the campus lab, got %T", m.top())
	}

	if other.labels.names != namesOff || other.labels.rates {
		t.Errorf("the choice was not kept for the next lab: %+v", other.labels)
	}

	press(m, "i")

	if other.labels.names != namesFocus || !strings.Contains(body(), "eth") {
		t.Errorf("the names did not come back:\n%s", body())
	}
}

// TestMapNamesTheLinkOfTwoNodes draws the smallest lab there is: two nodes next to each
// other. The link between them is given the length its names need, which the gap between
// two boxes alone does not have.
func TestMapNamesTheLinkOfTwoNodes(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	selectLab(t, m, "linux")
	press(m, "t")

	page, ok := m.top().(*mapPage)
	if !ok || page.lab != "linux" {
		t.Fatalf("expected the map of the linux lab, got %T", m.top())
	}

	link := ""

	for line := range strings.SplitSeq(ansi.Strip(m.render()), "\n") {
		if strings.Contains(line, "l1") && strings.Contains(line, "l2") {
			link = line
		}
	}

	if strings.Count(link, "eth1") != 2 { // one name at each end
		t.Fatalf("the link between the two nodes does not name its interfaces:\n%s",
			ansi.Strip(m.render()))
	}

	snapshot(t, m, "map_two_nodes_120x35")

	// Without names the nodes move together again.
	wide, _ := page.layout.Size()

	press(m, "i", "i")

	if narrow, _ := page.layout.Size(); narrow >= wide {
		t.Errorf("the drawing is %d wide without names and %d with them", narrow, wide)
	}

	// A window too small for the longer link gets the drawing without the room.
	page.labels.names = namesAll
	page.resize(30, 10) // barely room for the two boxes

	if w, _ := page.layout.Size(); w > 30 { // the width of the view
		t.Errorf("the drawing is %d wide in a view of 30", w)
	}
}

// clearNotes drops the boxes of the nodes from the lines of a map, whose second line
// holds figures that look like rates.
func clearNotes(lines []string) string {
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		if !strings.Contains(line, "%") {
			kept = append(kept, line)
		}
	}

	return strings.Join(kept, "\n")
}

func TestScreenMapNarrow(t *testing.T) {
	m, _ := newTestModel(t, 80, 24)
	press(m, "enter", "5")
	snapshot(t, m, "map_80x24")
}

// TestMapNavigation moves the focus across the map and opens what it lands on.
func TestMapNavigation(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	press(m, "enter", "5")

	page, ok := m.top().(*mapPage)
	if !ok {
		t.Fatalf("expected the map, got %T", m.top())
	}

	if page.lab != "dc1" || page.node == "" {
		t.Fatalf("the map shows lab %q with the focus on %q", page.lab, page.node)
	}

	start := page.node

	press(m, "down")

	if page.node == start {
		t.Errorf("moving down left the focus on %s", page.node)
	}

	press(m, "tab")

	if page.edge < 0 {
		t.Error("tab did not focus a link of the node")
	}

	if got := len(page.edgeInterfaces(true)); got == 0 {
		t.Error("the focused link has no interfaces to act on")
	}

	press(m, "esc")

	if _, still := m.top().(*mapPage); still && m.top().tab() != tabLabs {
		t.Errorf("escape should leave the map, the UI shows %T", m.top())
	}
}

func TestMapOfAnUndeployedLab(t *testing.T) {
	m, _ := newTestModel(t, 120, 35)
	selectLab(t, m, "clos")
	press(m, "t")

	page, ok := m.top().(*mapPage)
	if !ok || page.lab != "clos" {
		t.Fatalf("expected the map of clos, got %T", m.top())
	}

	screen := ansi.Strip(m.render())
	for _, want := range []string{"spine1", "leaf4", "not running"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the map of a lab that is not deployed does not show %q:\n%s", want, screen)
		}
	}
}

// TestScreenMapShapes draws the map of labs of different shapes: a service provider
// network, a ring and a campus.
func TestScreenMapShapes(t *testing.T) {
	for _, lab := range []string{"sp-core", "metro", "campus"} {
		be := clabinternaltuibackendfake.New(
			clabinternaltuibackendfake.Options{Now: fixedClock, Seed: 1},
		)
		m := New(be, Options{Now: fixedClock, Home: "/home/demo", Lab: lab})
		m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
		press(m, "5")
		snapshot(t, m, "map_"+lab+"_140x40")
	}
}
