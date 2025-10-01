package difflibgo

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	autoJunkLenHeuristic = 200
	insertOp             = 105
	deleteOp             = 100
	equalOp              = 101
	replaceOp            = 114
)

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func calculateRatio(matches, length int) float64 {
	if length > 0 {
		return 2.0 * float64(matches) / float64(length)
	}

	return 1.0
}

type Match struct {
	A    int
	B    int
	Size int
}

type OpCode struct {
	Tag    byte
	SeqALo int
	SeqAHi int
	SeqBLo int
	SeqBHi int
}

// SequenceMatcher is a port of the python standard library difflib.SequenceMatcher into go. The
// original class is here: https://github.com/python/cpython/blob/main/Lib/difflib.py#L44. This
// version only works to compare slices of strings, and removes the `junk` components of the python
// implementation.
type SequenceMatcher struct {
	sequenceA      []string
	sequenceB      []string
	matchingBlocks []Match
	opCodes        []OpCode

	// indices of things in b that are not junk; "b2j" in difflib
	bNonJunkIndicies map[string][]int

	// things deemed "auto junk" by the heuristic; "bpopular" in difflib
	bAutoJunk  map[string]struct{}
	fullBCount map[string]int
}

func (s *SequenceMatcher) setSequences(a, b []string) {
	s.setSequenceA(a)
	s.setSequenceB(b)
}

func (s *SequenceMatcher) setSequenceA(a []string) {
	if &a == &s.sequenceA {
		return
	}

	s.sequenceA = a
	s.matchingBlocks = nil
	s.opCodes = nil
}

func (s *SequenceMatcher) setSequenceB(b []string) {
	if &b == &s.sequenceB {
		return
	}

	s.sequenceB = b
	s.matchingBlocks = nil
	s.opCodes = nil
	s.fullBCount = nil

	s.purgeAutoJunk()
}

func (s *SequenceMatcher) purgeAutoJunkElement() {
	s.bNonJunkIndicies = map[string][]int{}

	seqB := s.sequenceB[0]

	for i, seq := range seqB {
		indices := s.bNonJunkIndicies[string(seq)]
		indices = append(indices, i)
		s.bNonJunkIndicies[string(seq)] = indices
	}

	autoJunk := map[string]struct{}{}

	n := len(seqB)

	if n < autoJunkLenHeuristic {
		return
	}

	ntest := n/100 + 1

	for seq, indices := range s.bNonJunkIndicies {
		if len(indices) > ntest {
			autoJunk[seq] = struct{}{}
		}
	}

	for seq := range autoJunk {
		delete(s.bNonJunkIndicies, seq)
	}

	s.bAutoJunk = autoJunk
}

func (s *SequenceMatcher) purgeAutoJunkSlice() {
	s.bNonJunkIndicies = map[string][]int{}

	for i, seq := range s.sequenceB {
		indices := s.bNonJunkIndicies[seq]
		indices = append(indices, i)
		s.bNonJunkIndicies[seq] = indices
	}

	autoJunk := map[string]struct{}{}

	n := len(s.sequenceB)

	if n < autoJunkLenHeuristic {
		return
	}

	ntest := n/100 + 1

	for seq, indices := range s.bNonJunkIndicies {
		if len(indices) > ntest {
			autoJunk[seq] = struct{}{}
		}
	}

	for seq := range autoJunk {
		delete(s.bNonJunkIndicies, seq)
	}

	s.bAutoJunk = autoJunk
}

func (s *SequenceMatcher) purgeAutoJunk() {
	if len(s.sequenceB) == 1 {
		s.purgeAutoJunkElement()
	} else {
		s.purgeAutoJunkSlice()
	}
}

func (s *SequenceMatcher) isBSeqJunk(seq string) bool {
	_, ok := s.bAutoJunk[seq]

	return ok
}

func (s *SequenceMatcher) findLongestMatchSingleElement(seqALo, seqAHi, seqBLo, seqBHi int) Match {
	besti, bestj, bestsize := seqALo, seqBLo, 0
	j2len := map[int]int{}

	seqA, seqB := s.sequenceA[0], s.sequenceB[0]

	for i := seqALo; i != seqAHi; i++ {
		newj2len := map[int]int{}

		for _, j := range s.bNonJunkIndicies[string(seqA[i])] {
			if j < seqBLo {
				continue
			}

			if j >= seqBHi {
				break
			}

			k := j2len[j-1] + 1
			newj2len[j] = k

			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}

		j2len = newj2len
	}

	for besti > seqALo && bestj > seqBLo && !s.isBSeqJunk(string(seqB[bestj-1])) &&
		string(seqA[besti-1]) == string(seqB[bestj-1]) {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}

	for besti+bestsize < seqAHi && bestj+bestsize < seqBHi &&
		!s.isBSeqJunk(string(seqB[bestj+bestsize])) &&
		string(seqA[besti+bestsize]) == string(seqB[bestj+bestsize]) {
		bestsize++
	}

	for besti > seqALo && bestj > seqBLo && s.isBSeqJunk(string(seqB[bestj-1])) &&
		string(seqA[besti-1]) == string(seqB[bestj-1]) {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}

	for besti+bestsize < seqAHi && bestj+bestsize < seqBHi &&
		s.isBSeqJunk(string(seqB[bestj+bestsize])) &&
		string(seqA[besti+bestsize]) == string(seqB[bestj+bestsize]) {
		bestsize++
	}

	return Match{A: besti, B: bestj, Size: bestsize}
}

func (s *SequenceMatcher) findLongestMatchSlice(seqALo, seqAHi, seqBLo, seqBHi int) Match {
	besti, bestj, bestsize := seqALo, seqBLo, 0
	j2len := map[int]int{}

	for i := seqALo; i != seqAHi; i++ {
		newj2len := map[int]int{}

		for _, j := range s.bNonJunkIndicies[s.sequenceA[i]] {
			if j < seqBLo {
				continue
			}

			if j >= seqBHi {
				break
			}

			k := j2len[j-1] + 1
			newj2len[j] = k

			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}

		j2len = newj2len
	}

	for besti > seqALo && bestj > seqBLo && !s.isBSeqJunk(s.sequenceB[bestj-1]) &&
		s.sequenceA[besti-1] == s.sequenceB[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}

	for besti+bestsize < seqAHi && bestj+bestsize < seqBHi &&
		!s.isBSeqJunk(s.sequenceB[bestj+bestsize]) &&
		s.sequenceA[besti+bestsize] == s.sequenceB[bestj+bestsize] {
		bestsize++
	}

	for besti > seqALo && bestj > seqBLo && s.isBSeqJunk(s.sequenceB[bestj-1]) &&
		s.sequenceA[besti-1] == s.sequenceB[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}

	for besti+bestsize < seqAHi && bestj+bestsize < seqBHi &&
		s.isBSeqJunk(s.sequenceB[bestj+bestsize]) &&
		s.sequenceA[besti+bestsize] == s.sequenceB[bestj+bestsize] {
		bestsize++
	}

	return Match{A: besti, B: bestj, Size: bestsize}
}

func (s *SequenceMatcher) findLongestMatch(seqALo, seqAHi, seqBLo, seqBHi int) Match {
	if len(s.sequenceA) == 1 && len(s.sequenceB) == 1 {
		return s.findLongestMatchSingleElement(seqALo, seqAHi, seqBLo, seqBHi)
	}

	return s.findLongestMatchSlice(seqALo, seqAHi, seqBLo, seqBHi)
}

func (s *SequenceMatcher) getMatchingBlocks() []Match {
	if s.matchingBlocks != nil {
		return s.matchingBlocks
	}

	var matchBlocks func(alo, ahi, blo, bhi int, matched []Match) []Match

	matchBlocks = func(seqALo, seqAHi, seqBLo, seqBHi int, matched []Match) []Match {
		match := s.findLongestMatch(seqALo, seqAHi, seqBLo, seqBHi)
		i, j, k := match.A, match.B, match.Size

		if match.Size > 0 {
			matched = append(matched, match)

			if seqALo < i && seqBLo < j {
				matched = matchBlocks(seqALo, i, seqBLo, j, matched)
			}

			if i+k < seqAHi && j+k < seqBHi {
				matched = matchBlocks(i+k, seqAHi, j+k, seqBHi, matched)
			}
		}

		return matched
	}

	la, lb := len(s.sequenceA), len(s.sequenceB)
	if len(s.sequenceA) == 1 && len(s.sequenceB) == 1 {
		la, lb = len(s.sequenceA[0]), len(s.sequenceB[0])
	}

	matched := matchBlocks(0, la, 0, lb, nil)

	sort.Slice(matched, func(i, j int) bool {
		// this *should*(?) match how the python implementation named tuple sorting works...
		if matched[i].A != matched[j].A {
			return matched[i].A < matched[j].A
		}

		if matched[i].B != matched[j].B {
			return matched[i].B < matched[j].B
		}

		return matched[i].Size < matched[j].Size
	})

	var nonAdjacent []Match

	i1, j1, k1 := 0, 0, 0

	for _, b := range matched {
		i2, j2, k2 := b.A, b.B, b.Size
		if i1+k1 == i2 && j1+k1 == j2 {
			k1 += k2
		} else {
			if k1 > 0 {
				nonAdjacent = append(nonAdjacent, Match{i1, j1, k1})
			}

			i1, j1, k1 = i2, j2, k2
		}
	}

	if k1 > 0 {
		nonAdjacent = append(nonAdjacent, Match{i1, j1, k1})
	}

	nonAdjacent = append(nonAdjacent, Match{la, lb, 0})
	s.matchingBlocks = nonAdjacent

	return s.matchingBlocks
}

func (s *SequenceMatcher) getOpcodes() []OpCode {
	if s.opCodes != nil {
		return s.opCodes
	}

	i, j := 0, 0
	matching := s.getMatchingBlocks()

	opCodes := make([]OpCode, 0, len(matching))

	for _, m := range matching {
		ai, bj, size := m.A, m.B, m.Size
		tag := byte(0)

		if i < ai && j < bj {
			tag = 'r'
		} else if i < ai {
			tag = 'd'
		} else if j < bj {
			tag = 'i'
		}

		if tag > 0 {
			opCodes = append(opCodes, OpCode{tag, i, ai, j, bj})
		}

		i, j = ai+size, bj+size

		if size > 0 {
			opCodes = append(opCodes, OpCode{'e', ai, i, bj, j})
		}
	}

	s.opCodes = opCodes

	return s.opCodes
}

func (s *SequenceMatcher) ratio() float64 {
	var la, lb int

	if len(s.sequenceA) == 1 && len(s.sequenceB) == 1 {
		la, lb = len(s.sequenceA[0]), len(s.sequenceB[0])
	} else {
		la, lb = len(s.sequenceA), len(s.sequenceB)
	}

	matches := 0
	for _, mb := range s.getMatchingBlocks() {
		matches += mb.Size
	}

	return calculateRatio(matches, la+lb)
}

func (s *SequenceMatcher) quickRatio() float64 {
	var matches, la, lb int

	if len(s.sequenceA) == 1 && len(s.sequenceB) == 1 {
		seqA, seqB := s.sequenceA[0], s.sequenceB[0]
		la, lb = len(seqA), len(seqB)

		if s.fullBCount == nil {
			s.fullBCount = map[string]int{}
			for _, x := range seqB {
				s.fullBCount[string(x)]++
			}
		}

		avail := map[string]int{}
		matches = 0

		for _, x := range seqA {
			n, ok := avail[string(x)]
			if !ok {
				n = s.fullBCount[string(x)]
			}

			avail[string(x)] = n - 1

			if n > 0 {
				matches++
			}
		}
	} else {
		la, lb = len(s.sequenceA), len(s.sequenceB)

		if s.fullBCount == nil {
			s.fullBCount = map[string]int{}
			for _, x := range s.sequenceB {
				s.fullBCount[x]++
			}
		}

		avail := map[string]int{}
		matches = 0
		for _, x := range s.sequenceA {
			n, ok := avail[x]
			if !ok {
				n = s.fullBCount[x]
			}
			avail[x] = n - 1
			if n > 0 {
				matches++
			}
		}
	}

	return calculateRatio(matches, la+lb)
}

func (s *SequenceMatcher) realQuickRatio() float64 {
	var la, lb int

	// different than python because we must have slices of strings, so if slice is len 1
	// we can just use the length of the zeroith element
	if len(s.sequenceA) == 1 && len(s.sequenceB) == 1 {
		la, lb = len(s.sequenceA[0]), len(s.sequenceB[0])
	} else {
		la, lb = len(s.sequenceA), len(s.sequenceB)
	}

	return calculateRatio(min(la, lb), la+lb)
}

type Differ struct{}

func (d *Differ) fancyHelper(seqALo, seqAHi, seqBLo, seqBHi int, seqA, seqB []string) []string {
	var g []string

	if seqALo < seqAHi {
		if seqBLo < seqBHi {
			g = d.fancyReplace(seqALo, seqAHi, seqBLo, seqBHi, seqA, seqB)
		} else {
			g = d.dump("-", seqA, seqALo, seqAHi)
		}
	} else if seqBLo < seqBHi {
		g = d.dump("+", seqB, seqBLo, seqBHi)
	}

	return g
}

func (d *Differ) dump(tag string, sequence []string, lo, hi int) []string {
	var dumper []string

	for i := lo; i < hi; i++ {
		dumper = append(dumper, fmt.Sprintf("%s %s", tag, sequence[i]))
	}

	return dumper
}

func keepOriginalWs(s, tags string) string {
	var strippedS string

	var iterLen int

	if len(s) > len(tags) {
		iterLen = len(tags)
	} else {
		iterLen = len(s)
	}

	for i := 0; i < iterLen; i++ {
		c, tagC := s[i], tags[i]

		if string(tagC) == " " && unicode.IsSpace(rune(c)) {
			strippedS += string(c)
		} else {
			strippedS += string(tagC)
		}
	}

	return strings.TrimRight(strippedS, " ")
}

func (d *Differ) qFormat(aline, bline, atags, btags string) []string {
	var f []string

	atags = keepOriginalWs(aline, atags)
	btags = keepOriginalWs(bline, btags)

	f = append(f, fmt.Sprintf("- %s", aline))

	if len(atags) > 0 {
		f = append(f, fmt.Sprintf("? %s\n", atags))
	}

	f = append(f, fmt.Sprintf("+ %s", bline))

	if len(btags) > 0 {
		f = append(f, fmt.Sprintf("? %s\n", btags))
	}

	return f
}

func (d *Differ) plainReplace(seqALo, seqAHi, seqBLo, seqBHi int, seqA, seqB []string) []string {
	var first []string

	var second []string

	if seqBHi-seqBLo < seqAHi-seqALo {
		first = d.dump("+", seqB, seqBLo, seqBHi)
		second = d.dump("-", seqA, seqALo, seqAHi)
	} else {
		first = d.dump("-", seqA, seqALo, seqAHi)
		second = d.dump("+", seqB, seqBLo, seqBHi)
	}

	return append(first, second...)
}

func assembleFancyReplaceOutput(
	preSyncPointDiffs, formattedTags, postSyncPointDiffs []string,
) []string {
	var finalOut []string

	finalOut = append(finalOut, preSyncPointDiffs...)
	finalOut = append(finalOut, formattedTags...)
	finalOut = append(finalOut, postSyncPointDiffs...)

	return finalOut
}

func (d *Differ) fancyReplace(seqALo, seqAHi, seqBLo, seqBHi int, seqA, seqB []string) []string {
	bestRatio, cutoffRatio := 0.74, 0.75
	eqi, eqj := -1, -1
	bestI, bestJ := -1, -1

	s := &SequenceMatcher{}

	for j := seqBLo; j < seqBHi; j++ {
		bj := seqB[j]

		s.setSequenceB([]string{bj})

		for i := seqALo; i < seqAHi; i++ {
			ai := seqA[i]

			if ai == bj {
				if eqi == -1 {
					eqi, eqj = i, j
				}

				continue
			}

			s.setSequenceA([]string{ai})

			if s.realQuickRatio() > bestRatio && s.quickRatio() > bestRatio &&
				s.ratio() > bestRatio {
				bestRatio = s.ratio()
				bestI, bestJ = i, j
			}
		}
	}

	if bestRatio < cutoffRatio {
		if eqi == -1 {
			replaced := d.plainReplace(seqALo, seqAHi, seqBLo, seqBHi, seqA, seqB)
			return replaced
		}

		bestI, bestJ = eqi, eqj

		// no idea why it thinks this is ineffectual? is it?!
		bestRatio = 1.0 //nolint:ineffassign
	} else {
		eqi = -1
	}

	preSyncPointDiffs := d.fancyHelper(seqALo, bestI, seqBLo, bestJ, seqA, seqB)

	aelt, belt := seqA[bestI], seqB[bestJ]

	var formattedTags []string

	if eqi == -1 {
		atags, btags := "", ""

		s.setSequences([]string{aelt}, []string{belt})

		opCodes := s.getOpcodes()
		for _, opCode := range opCodes {
			la, lb := opCode.SeqAHi-opCode.SeqALo, opCode.SeqBHi-opCode.SeqBLo

			switch opCode.Tag {
			case replaceOp:
				atags += strings.Repeat("^", la)
				btags += strings.Repeat("^", lb)
			case deleteOp:
				atags += strings.Repeat("-", la)
			case insertOp:
				btags += strings.Repeat("+", lb)
			case equalOp:
				atags += strings.Repeat(" ", la)
				btags += strings.Repeat(" ", lb)
			default:
				panic("unknown opcode, this shouldn't happen...")
			}
		}

		formattedTags = d.qFormat(aelt, belt, atags, btags)
	} else {
		formattedTags = []string{fmt.Sprintf("  %s", aelt)}
	}

	postSyncPointDiffs := d.fancyHelper(seqALo+1, seqAHi, seqBLo+1, seqBHi, seqA, seqB)

	return assembleFancyReplaceOutput(preSyncPointDiffs, formattedTags, postSyncPointDiffs)
}

func (d *Differ) Compare(seqA, seqB []string) []string {
	s := &SequenceMatcher{}
	s.setSequences(
		seqA,
		seqB,
	)

	opCodes := s.getOpcodes()

	var finalOut []string

	for _, opCode := range opCodes {
		switch opCode.Tag {
		case replaceOp:
			c := d.fancyReplace(
				opCode.SeqALo,
				opCode.SeqAHi,
				opCode.SeqBLo,
				opCode.SeqBHi,
				seqA,
				seqB,
			)
			finalOut = append(finalOut, c...)
		case deleteOp:
			c := d.dump("-", seqA, opCode.SeqALo, opCode.SeqAHi)
			finalOut = append(finalOut, c...)
		case insertOp:
			c := d.dump("+", seqB, opCode.SeqBLo, opCode.SeqBHi)
			finalOut = append(finalOut, c...)
		case equalOp:
			c := d.dump(" ", seqA, opCode.SeqALo, opCode.SeqAHi)
			finalOut = append(finalOut, c...)
		default:
			panic("unknown opcode, this shouldn't happen...")
		}
	}

	return finalOut
}
