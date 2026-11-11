// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strings"

	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// nodePaneLines is the height of the details below the node list when the window is
	// drawn with the pixel layer, and pixelPaneMin the body height it takes to show them.
	nodePaneLines = 15
	pixelPaneMin  = 23

	cardRound   = 9
	buttonRound = 6
	tileRound   = 8
)

func (p *nodesPage) pixelPane() bool {
	return !p.hidePane && p.height >= pixelPaneMin
}

func (p *nodesPage) viewPixels(m *Model) string {
	sc := m.scene()
	if sc == nil {
		sc = clabinternaltuigfx.NewScene(m.metrics())
	}

	if !p.pixelPane() {
		return p.tbl.View()
	}

	// A blank line keeps the cards of the pane clear of the card of the table.
	pane := p.nodePane(m, sc, pixelBodyTop+p.height-nodePaneLines+1)

	return p.tbl.View() + "\n\n" + strings.Join(pane, "\n")
}

// kindIcon picks the icon that stands for a kind of node.
func kindIcon(kind string) clabinternaltuigfx.Icon {
	switch {
	case kind == "linux" || strings.Contains(kind, "host"):
		return clabinternaltuigfx.IconHost
	case strings.Contains(kind, "bridge") || strings.Contains(kind, "ovs"):
		return clabinternaltuigfx.IconSwitch
	default:
		return clabinternaltuigfx.IconRouter
	}
}

// nodePane draws the details of the node under the cursor as two cards: what it is and
// what can be done with it on the left, how busy it is on the right. top is the screen
// line the pane starts at.
//
//nolint:mnd,funlen // the layout of the cards, line by line
func (p *nodesPage) nodePane(m *Model, sc *clabinternaltuigfx.Scene, top int) []string {
	sk := m.skin()
	y := float64(top)
	mid := m.width / 2
	leftEnd := mid - 2
	height := float64(nodePaneLines) - 0.8

	sc.Box(1, y+0.3, float64(mid)-1.5, height, cardRound, sk.Card, sk.CardEdge, 1)
	sc.Box(float64(mid), y+0.3, float64(m.width-1-mid), height, cardRound, sk.Card, sk.CardEdge, 1)

	rows := make([]textRow, nodePaneLines-1)

	n := p.current()
	if n == nil {
		rows[1].at(3, m.th.Muted.Render("no node selected"))

		return rowLines(rows, m.width)
	}

	// The left card: what the node is.
	hue := clabinternaltuigfx.RGB(m.th.Hue(n.Kind))
	sc.Box(3, y+1.05, 4.6, 1.9, tileRound, hue.Alpha(tintAlpha), hue.Alpha(0.4), 1)
	sc.Icon(kindIcon(n.Kind), 5.3, y+2, 22, 1.5, hue)

	readiness := n.Readiness()
	_, level := readinessLook(readiness)
	state := readiness.String()
	tint := clabinternaltuigfx.RGB(m.th.LevelStyle(level).GetForeground())
	stateCol := leftEnd - clabinternaltuiui.Width(state) - 1

	rows[1].at(9, m.th.Bold.Render(clabinternaltuiui.Truncate(
		clabinternaltuiui.Sanitize(n.Container), stateCol-12)))
	sc.Fill(float64(stateCol)-0.8, y+1.08, float64(clabinternaltuiui.Width(state))+1.6, 0.84,
		chipRound, tint.Alpha(tintAlpha))
	rows[1].at(stateCol, m.th.LevelStyle(level).Render(state))

	sub := clabinternaltuiui.Sanitize(n.Kind)
	if n.Status != "" {
		sub += " · " + n.Status
	}

	rows[2].at(9, m.th.Muted.Render(clabinternaltuiui.Truncate(sub, leftEnd-9)))
	sc.HLine(1, y+3.5, float64(mid)-1.5, sk.CardEdge)

	shell := ""
	if methods := m.be.ConnectMethods(n); len(methods) > 0 {
		shell = methods[0].Label
	}

	fields := [][2]string{
		{"image", clabinternaltuiui.Sanitize(n.Image)},
		{"ipv4", n.IPv4},
		{"ipv6", n.IPv6},
		{"ports", clabinternaltuiui.Ports(n.Ports)},
		{"shell", shell},
		{"group", clabinternaltuiui.Sanitize(n.Group)},
		{"id", n.ID},
	}

	for i, f := range fields {
		value, style := f[1], m.th.Text
		if value == "" {
			value, style = clabinternaltuiui.Dash, m.th.Faint
		}

		rows[4+i].at(3, m.th.Muted.Render(f[0])).
			at(12, style.Render(clabinternaltuiui.Truncate(value, leftEnd-12)))
	}

	p.nodeButtons(m, sc, &rows[12], top+12, leftEnd)

	// The right card: how busy the node is.
	x := mid + 2
	chartX, chartW := float64(x), float64(m.width-3-x)
	accent := sk.Accent

	rows[1].at(
		x,
		m.th.Muted.Render("CPU")+" "+m.th.Bold.Render(clabinternaltuiui.Percent(n.CPUPercent)),
	)
	chartGrid(sc, chartX, y+2.3, chartW, 3.9, sk)
	sc.Chart(
		chartX,
		y+2.3,
		chartW,
		3.9,
		n.CPUSeries,
		cpuLimit(n.CPUSeries, 1)*1.15,
		1.6,
		accent,
		0.35,
	)

	rxColor := clabinternaltuigfx.RGB(m.th.Palette.Info)
	txColor := clabinternaltuigfx.RGB(m.th.Palette.Hues[1%len(m.th.Palette.Hues)])

	rows[7].at(x, m.th.Muted.Render("Traffic")).
		at(x+12, m.th.Text.Render("rx "+clabinternaltuiui.Rate(n.RxBps))).
		at(x+24, m.th.Text.Render("tx "+clabinternaltuiui.Rate(n.TxBps)))
	sc.Dot(float64(x)+10.5, y+7.5, 3.5, rxColor)
	sc.Dot(float64(x)+22.5, y+7.5, 3.5, txColor)

	limit := 0.0
	for _, v := range append(append([]float64{}, n.RxSeries...), n.TxSeries...) {
		limit = max(limit, v)
	}

	chartGrid(sc, chartX, y+8.3, chartW, 5, sk)
	sc.Chart(chartX, y+8.3, chartW, 5, n.RxSeries, limit*1.15, 1.6, rxColor, 0.28)
	sc.Chart(chartX, y+8.3, chartW, 5, n.TxSeries, limit*1.15, 1.6, txColor, 0.2)

	return rowLines(rows, m.width)
}

// chartGrid draws the faint horizontal lines behind a chart.
func chartGrid(sc *clabinternaltuigfx.Scene, x, y, w, h float64, sk *clabinternaltuiui.Skin) {
	const lines = 3

	for i := 0; i <= lines; i++ {
		sc.HLine(x, y+h*float64(i)/lines, w, sk.RowLine)
	}
}

// rowLines returns the lines of a block that was laid out in columns of the window, for a
// page: its lines start where the body does.
func rowLines(rows []textRow, width int) []string {
	lines := make([]string, len(rows))
	for i := range rows {
		lines[i] = rows[i].line(width)[pixelInset:]
	}

	return lines
}

// nodeButtons draws the actions of the node as buttons, as many as fit. They do what
// their key does, also when clicked.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (p *nodesPage) nodeButtons(
	m *Model, sc *clabinternaltuigfx.Scene, row *textRow, line, end int,
) {
	sk := m.skin()
	_, list := p.actions(m)
	x := 3
	first := true

	for i := range list {
		a := &list[i]
		if a.off != "" {
			continue
		}

		label := title(a.name)
		// A cell of padding on each side, and two between the label and the key.
		w := clabinternaltuiui.Width(label) + clabinternaltuiui.Width(a.key) + 4
		if x+w > end {
			break
		}

		fx, fy := float64(x), float64(line)-0.3 // around the text line

		//nolint:mnd // the button is a little taller than the line of its label
		if first {
			sc.Shadow(fx, fy, float64(w), 1.6, buttonRound, 7, 0, sk.Accent.Alpha(0.35))
			sc.Fill(fx, fy, float64(w), 1.6, buttonRound, sk.Accent)
			// The cells get the color of the button as well: it is hidden under the
			// pixels, but it is what the terminal measures the contrast of the text
			// against.
			on := m.th.ChipActive.UnsetPadding()
			row.at(x+1, on.Render(label+"  ")+on.Bold(false).Render(a.key))
		} else {
			sc.Box(fx, fy, float64(w), 1.6, buttonRound, sk.Raised, sk.RaisedEdge, 1)
			row.at(x+1, m.th.Text.Render(label)+"  "+m.th.Muted.Render(a.key))
		}

		if key, ok := hintKey(hint{key: a.key}); ok {
			m.px.zones = append(
				m.px.zones,
				pixelZone{rect: rect{x: x, y: line, w: w, h: 1}, key: key},
			)
		}

		first = false
		x += w + 1
	}
}
