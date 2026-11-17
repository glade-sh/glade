package dml

// The flat Keyboard Distance of the custom Last Name method (clean-room fit
// to the native rows of duplicate_fuzzy_panels k1 to k3, hk1, hk2, tk01 to
// tk22 and bk01 to bk09; ck01 to ck09 held it out, rk01 to rk08 and sk01 to
// sk08 are later blind panels).
//
// The names are compared position by position over the shorter length; there
// is no alignment. A differing position costs a number of key steps: the
// column distance, plus two between the top and the bottom row. Every step
// costs keyboardFlatStep, whatever the position and the pair of keys (two
// letters zero steps apart cost keyboardFlatColumn), and
// every trailing character costs keyboardFlatTrail. The cost is divided by
// the longer length and the score is 100 * (1 - cost). The confidence is
// (100 + score) / 2.
//
// keyboardDistance, the per-key-pair score fitted earlier, still decides the
// pairs it matched in a gated shape: those native matches and their
// confidences stay. The flat score adds the matches that table misses, in
// every shape, at the same keyboardFallbackMargin. No captured native
// non-match reaches it; the highest scores 92.50, 0.50 below the margin.
const (
	keyboardFlatStep  = 0.0759
	keyboardFlatTrail = 0.2437
	// keyboardFlatColumn is the cost of a substitution that is zero key steps
	// (the same column, one row apart): the base scorer's neighbour cost. A
	// zero cost matched names native does not (rkeyboard, 14 pairs at 93.01 to
	// 93.50).
	keyboardFlatColumn = 0.05
)

// keyboardFlatPos is the key position of each letter, row then column.
var keyboardFlatPos = func() (pos [26][2]int) {
	for c := 'a'; c <= 'z'; c++ {
		pos[c-'a'] = qwertyPosition[c]
	}
	return pos
}()

// keyboardFlatSteps is the key steps between two letter indexes: the column
// distance, plus two between the top and the bottom row.
func keyboardFlatSteps(x, y int) int {
	px, py := keyboardFlatPos[x], keyboardFlatPos[y]
	steps := abs(px[1] - py[1])
	if abs(px[0]-py[0]) == 2 {
		steps += 2
	}
	return steps
}

// keyboardFlatCost is the cost of two different letters: key steps, or
// keyboardFlatColumn when they are zero steps apart.
func keyboardFlatCost(x, y int) float64 {
	if steps := keyboardFlatSteps(x, y); steps > 0 {
		return keyboardFlatStep * float64(steps)
	}
	return keyboardFlatColumn
}

// keyboardFlatDistance is the flat Keyboard Distance score of two folded last
// names. Two letters cost by key steps and a trailing letter costs
// keyboardFlatTrail; an accent alone, a space or a punctuation mark keep the
// costs of keyboardDistance, and ok is false where a digit or a letter that is
// not on the keyboard would differ.
func keyboardFlatDistance(a, b string) (float64, bool) {
	if score, ok, handled := keyboardFlatASCII(a, b); handled {
		return score, ok
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) > len(rb) {
		ra, rb = rb, ra
	}
	n := len(rb)
	if n == 0 {
		return 100, true
	}
	cost, different := 0.0, false
	for i := range ra {
		if ra[i] == rb[i] {
			continue
		}
		different = true
		x, y := keyboardCode(ra[i]), keyboardCode(rb[i])
		if x < 26 && y < 26 && x != y {
			cost += keyboardFlatCost(int(x), int(y))
			continue
		}
		cost += keyboardPair[0][x][y]
	}
	trail := &keyboardTrail[0]
	if !different {
		trail = &keyboardTrail[1]
	}
	for _, r := range rb[len(ra):] {
		code := keyboardCode(r)
		if code == keyboardOther && n-len(ra) > 2 {
			return 0, false
		}
		if code < 26 {
			cost += keyboardFlatTrail
		} else {
			cost += trail[code]
		}
	}
	if cost >= keyboardNever {
		return 0, false
	}
	return 100 * (1 - cost/float64(n)), true
}

// keyboardFlatASCII is keyboardFlatDistance without allocation for two
// lower-case letter strings; handled is false for any other character.
func keyboardFlatASCII(a, b string) (score float64, ok, handled bool) {
	if len(a) > len(b) {
		a, b = b, a
	}
	n := len(b)
	if n == 0 {
		return 0, false, false
	}
	cost := 0.0
	for i := 0; i < n; i++ {
		if b[i] < 'a' || b[i] > 'z' || i < len(a) && (a[i] < 'a' || a[i] > 'z') {
			return 0, false, false
		}
		if i < len(a) && a[i] != b[i] {
			cost += keyboardFlatCost(int(a[i]-'a'), int(b[i]-'a'))
		}
	}
	cost += float64(keyboardFlatTrail * float64(n-len(a)))
	return 100 * (1 - cost/float64(n)), true, true
}
