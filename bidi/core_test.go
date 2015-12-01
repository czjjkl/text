// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bidi

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/internal/gen"
	"golang.org/x/text/internal/ucd"
	"golang.org/x/text/unicode/norm"
)

var testLevels = flag.Bool("levels", false, "enable testing of levels")

// TestBidiCore performs the tests in BidiTest.txt.
// See http://www.unicode.org/Public/UCD/latest/ucd/BidiTest.txt.
func TestBidiCore(t *testing.T) {
	if !*long {
		return
	}
	r := gen.OpenUCDFile("BidiTest.txt")
	defer r.Close()

	var wantLevels, wantOrder []string
	p := ucd.New(r, ucd.Part(func(p *ucd.Parser) {
		s := strings.Split(p.String(0), ":")
		switch s[0] {
		case "Levels":
			wantLevels = strings.Fields(s[1])
		case "Reorder":
			wantOrder = strings.Fields(s[1])
		default:
			log.Fatalf("Unknown part %q.", s[0])
		}
	}))

	for p.Next() {
		types := []class{}
		for _, s := range p.Strings(0) {
			types = append(types, bidiClass[s])
		}
		// We ignore the bracketing part of the algorithm.
		pairTypes := make([]bracketType, len(types))
		pairValues := make([]rune, len(types))

		for i := uint(0); i < 3; i++ {
			if p.Uint(1)&(1<<i) == 0 {
				continue
			}
			lev := level(int(i) - 1)
			par := newParagraph(types, pairTypes, pairValues, lev)

			if *testLevels {
				levels := par.resultLevels
				for i, s := range wantLevels {
					if s == "x" {
						continue
					}
					l, _ := strconv.ParseUint(s, 10, 8)
					if level(l)&1 != levels[i]&1 {
						t.Errorf("%s:%d:levels: got %v; want %v", p.String(0), lev, levels, wantLevels)
						break
					}
				}
			}

			order := par.getReordering([]int{len(types)})
			gotOrder := filterOrder(types, order)
			if got, want := fmt.Sprint(gotOrder), fmt.Sprint(wantOrder); got != want {
				t.Errorf("%s:%d:order: got %v; want %v\noriginal %v", p.String(0), lev, got, want, order)
			}
		}
	}
	if err := p.Err(); err != nil {
		log.Fatal(err)
	}
}

var removeClasses = map[class]bool{
	_LRO: true,
	_RLO: true,
	_RLE: true,
	_LRE: true,
	_PDF: true,
	_BN:  true,
}

// TestBidiCharacters performs the tests in BidiCharacterTest.txt.
// See http://www.unicode.org/Public/UCD/latest/ucd/BidiCharacterTest.txt
func TestBidiCharacters(t *testing.T) {
	if !*long {
		return
	}
	parse("BidiCharacterTest.txt", func(p *ucd.Parser) {
		var (
			types      []class
			pairTypes  []bracketType
			pairValues []rune
			parLevel   level

			wantLevel       = level(p.Int(2))
			wantLevels      = p.Strings(3)
			wantVisualOrder = p.Strings(4)
		)

		switch l := p.Int(1); l {
		case 0, 1:
			parLevel = level(l)
		case 2:
			parLevel = implicitLevel
		default:
			// Spec says to ignore unknown parts.
		}

		trie := newBidiTrie(0)
		runes := p.Runes(0)

		for _, r := range runes {
			// Assign the bracket type.
			if d := norm.NFKD.PropertiesString(string(r)).Decomposition(); d != nil {
				r = []rune(string(d))[0]
			}
			e, _ := trie.lookupString(string(r))
			entry := entry(e)

			// Assign the class for this rune.
			types = append(types, entry.class(r))

			switch {
			case !entry.isBracket():
				pairTypes = append(pairTypes, bpNone)
				pairValues = append(pairValues, 0)
			case entry.isOpen():
				pairTypes = append(pairTypes, bpOpen)
				pairValues = append(pairValues, r)
			default:
				pairTypes = append(pairTypes, bpClose)
				pairValues = append(pairValues, entry.reverseBracket(r))
			}
		}
		par := newParagraph(types, pairTypes, pairValues, parLevel)

		// Test results:
		if got := par.embeddingLevel; got != wantLevel {
			t.Errorf("%v:level: got %d; want %d", string(runes), got, wantLevel)
		}

		if *testLevels {
			gotLevels := getLevelStrings(types, par.resultLevels)
			if got, want := fmt.Sprint(gotLevels), fmt.Sprint(wantLevels); got != want {
				t.Errorf("%04X %q:%d: got %v; want %v\nval: %x\npair: %v", runes, string(runes), parLevel, got, want, pairValues, pairTypes)
			}
		}

		order := par.getReordering([]int{len(types)})
		order = filterOrder(types, order)
		if got, want := fmt.Sprint(order), fmt.Sprint(wantVisualOrder); got != want {
			t.Errorf("%04X %q:%d: got %v; want %v\ngot order: %s", runes, string(runes), parLevel, got, want, reorder(runes, order))
		}
	})
}

func getLevelStrings(cl []class, levels []level) []string {
	var results []string
	for i, l := range levels {
		if !removeClasses[cl[i]] {
			results = append(results, fmt.Sprint(l))
		} else {
			results = append(results, "x")
		}
	}
	return results
}

func filterOrder(cl []class, order []int) []int {
	no := []int{}
	for _, o := range order {
		if !removeClasses[cl[o]] {
			no = append(no, o)
		}
	}
	return no
}

func reorder(r []rune, order []int) string {
	nr := make([]rune, len(order))
	for i, o := range order {
		nr[i] = r[o]
	}
	return string(nr)
}

// bidiClass names and codes taken from class "bc" in
// http://www.unicode.org/Public/8.0.0/ucd/PropertyValueAliases.txt
var bidiClass = map[string]class{
	"AL":  _AL,  // classArabicLetter,
	"AN":  _AN,  // classArabicNumber,
	"B":   _B,   // classParagraphSeparator,
	"BN":  _BN,  // classBoundaryNeutral,
	"CS":  _CS,  // classCommonSeparator,
	"EN":  _EN,  // classEuropeanNumber,
	"ES":  _ES,  // classEuropeanSeparator,
	"ET":  _ET,  // classEuropeanTerminator,
	"L":   _L,   // classLeftToRight,
	"NSM": _NSM, // classNonspacingMark,
	"ON":  _ON,  // classOtherNeutral,
	"R":   _R,   // classRightToLeft,
	"S":   _S,   // classSegmentSeparator,
	"WS":  _WS,  // classWhiteSpace,

	"LRO": _LRO, // classLeftToRightOverride,
	"RLO": _RLO, // classRightToLeftOverride,
	"LRE": _LRE, // classLeftToRightEmbedding,
	"RLE": _RLE, // classRightToLeftEmbedding,
	"PDF": _PDF, // classPopDirectionalFormat,
	"LRI": _LRI, // classLeftToRightIsolate,
	"RLI": _RLI, // classRightToLeftIsolate,
	"FSI": _FSI, // classFirstStrongIsolate,
	"PDI": _PDI, // classPopDirectionalIsolate,
}
