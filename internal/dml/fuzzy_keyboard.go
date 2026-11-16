package dml

import (
	"strings"
	"unicode"
)

// Keyboard Distance of the custom Last Name method, fitted to the native rows
// of duplicate_fuzzy_matching and fuzzy_name_keys (ln1, suf1, pre1, rsp1, ks1
// to ks3; 2,528 distinct pairs that reach this step).
//
// The names are compared position by position over the shorter length; there
// is no alignment; its caller applies it only to the joint (length,
// substitutions, trailing) shapes where a native match needs it, and only
// above the score of any native non-match of the shape (keyboardPanelShape,
// keyboardPanelFloor;
// "Farnsdelwick" finds "Farnsdraeovk" at 96 with five substitutions; "Abernathy" finds "Abernaty" at 99 as h/y plus a trailing
// y). The cost is divided by the longer length, the score is 100 * (1 - cost)
// and a score of 90 matches.
//
//   - A substitution costs by key distance: the column distance, plus two
//     between the top and the bottom row. Neighbouring keys cost 0.06 and
//     further keys 0.079 a step. A substitution of the final character of two
//     names of equal length is one step cheaper (0.05, then 0.079 for every
//     further step). The custom scorer keeps the base score for a single
//     substitution, including "Harlow II" / "Harlow IV" (98; native 97).
//   - The rows do not follow the key distance exactly. keyboardFittedInner
//     and keyboardFittedFinal list the key pairs whose rows need another
//     cost; every other pair keeps the formula.
//   - An accent alone costs 0.16. A letter against a space or a punctuation
//     mark, and a space against a punctuation mark, have their own costs
//     ("Harlow Jr" finds "Harlowe" at 96). Digits never substitute.
//   - A trailing character costs by the character. After an otherwise equal
//     name the costs sit near 0.25 (keyboardExtension: "Martinez" finds
//     "Martinezk" at 99 and "Martinezkmp" at 97, "Frank" does not find
//     "Franklin"). After a substitution the top row is cheapest and the
//     bottom row dearest (keyboardTrailing).
//
// The fit keeps every native non-match at least 0.3 below the threshold. It
// reproduces the membership of those rows. Some confidences differ by one
// point, including where rows with the same key pairs disagree (Glade reports
// "Abernathy" and "Abernaty" at 98); legacy matches retain their old scores.
const (
	keyboardAccent      = 0.16
	keyboardLetterSpace = 0.169
	keyboardLetterPunct = 0.107
	keyboardSpacePunct  = 0.082
	keyboardNever       = 1000.0

	keyboardSpace = 26
	keyboardPunct = 27
	keyboardOther = 28
	keyboardCodes = 29
)

// Costs in thousandths for the key pairs that leave the formula, inner
// positions first, then the final position of two names of equal length.
const (
	keyboardFittedInner = "ad106 ae082 ah379 aj499 al681 ao612 ap688 ar229 as055 az011 by250 ck360 " +
		"cs050 cv055 cx055 dg120 dh227 dl434 dt170 dz154 ei406 ek455 en399 eo492 " +
		"ep547 eq102 er055 et144 eu304 ew105 ey262 ez284 fj292 fs166 gq256 gu195 " +
		"hi212 hj110 hl227 ht090 hy025 ij136 il101 in304 io055 iq514 ir286 is482 " +
		"it244 iv422 iw461 iy145 jl172 jr168 js424 kl008 kn142 kr353 ls533 lu053 " +
		"lz623 mn101 no402 nx341 op055 oq613 or341 ou126 ow575 oy262 pt424 pu218 " +
		"qr162 qs055 qt309 qx150 rs182 rt037 rv088 sw027 sx055 sz041 tw206 vx151 " +
		"wz201 xy465"
	keyboardFittedFinal = "ae093 al568 co568 dt084 es055 et227 ew055 ez248 gk206 hi111 iq484 it283 " +
		"iv464 iw510 ix568 kl055 km020 ks425 lq568 lz568 mn038 oq568 ox641 oz721 " +
		"pq641 ps568 pv568 pw568 px721 qt245 sz042 tw283 ux484 uz568"
)

// keyboardTrailing is the cost of a trailing letter, a to z. No row has a
// matched trailing c, q, u, w or x; those take their row's cost, raised where
// a native non-match needs it.
var keyboardTrailing = [26]float64{
	0.347, 0.334, 0.35, 0.278, 0.158, 0.27, 0.279, 0.287, 0.262, 0.209, 0.242, 0.292, 0.369,
	0.38, 0.194, 0.216, 0.21, 0.203, 0.272, 0.247, 0.21, 0.376, 0.21, 0.35, 0.262, 0.334,
}

// keyboardExtension is the cost of a trailing letter, a to z, after an
// otherwise equal name. A letter with no row costs 0.25.
var keyboardExtension = [26]float64{
	0.268, 0.25, 0.25, 0.217, 0.158, 0.25, 0.255, 0.25, 0.336, 0.278, 0.256, 0.233, 0.256,
	0.256, 0.228, 0.205, 0.25, 0.361, 0.255, 0.159, 0.228, 0.25, 0.25, 0.25, 0.25, 0.255,
}

const (
	keyboardTrailingSpace  = 0.508
	keyboardTrailingPunct  = 0.46
	keyboardExtensionSpace = 0.341
	keyboardExtensionPunct = 0.129
	// keyboardExtensionOther is the cost of a trailing digit, or of a letter
	// that is not on the keyboard, after an otherwise equal name. No row
	// measures it: one or two keep the earlier 0.25, more never match.
	keyboardExtensionOther = 0.25
)

// keyboardPair[0] holds the substitution cost of two key codes at an inner
// position, keyboardPair[1] at the final position. keyboardTrail[0] holds the
// cost of a trailing key code after a substitution, keyboardTrail[1] after an
// otherwise equal name.
var keyboardPair, keyboardTrail = func() (pair [2][keyboardCodes][keyboardCodes]float64, trail [2][keyboardCodes]float64) {
	for final := range pair {
		for x := range pair[final] {
			for y := range pair[final][x] {
				pair[final][x][y] = keyboardNever
			}
		}
		for x := 'a'; x <= 'z'; x++ {
			for y := 'a'; y <= 'z'; y++ {
				px, py := qwertyPosition[x], qwertyPosition[y]
				steps := abs(px[1] - py[1])
				if abs(px[0]-py[0]) == 2 {
					steps += 2
				}
				cost := keyboardAccent
				switch {
				case x == y:
				case final == 1 && steps <= 1:
					cost = 0.05
				case final == 1:
					cost = 0.079 * float64(steps-1)
				case steps <= 1:
					cost = 0.06
				default:
					cost = 0.079 * float64(steps)
				}
				pair[final][x-'a'][y-'a'] = cost
			}
			pair[final][x-'a'][keyboardSpace], pair[final][keyboardSpace][x-'a'] = keyboardLetterSpace, keyboardLetterSpace
			pair[final][x-'a'][keyboardPunct], pair[final][keyboardPunct][x-'a'] = keyboardLetterPunct, keyboardLetterPunct
		}
		pair[final][keyboardSpace][keyboardPunct], pair[final][keyboardPunct][keyboardSpace] = keyboardSpacePunct, keyboardSpacePunct
	}
	for final, fitted := range []string{keyboardFittedInner, keyboardFittedFinal} {
		for _, entry := range strings.Fields(fitted) {
			x, y := entry[0]-'a', entry[1]-'a'
			cost := float64(int(entry[2]-'0')*100+int(entry[3]-'0')*10+int(entry[4]-'0')) / 1000
			pair[final][x][y], pair[final][y][x] = cost, cost
		}
	}
	copy(trail[0][:], keyboardTrailing[:])
	trail[0][keyboardSpace], trail[0][keyboardPunct], trail[0][keyboardOther] = keyboardTrailingSpace, keyboardTrailingPunct, keyboardNever
	copy(trail[1][:], keyboardExtension[:])
	trail[1][keyboardSpace], trail[1][keyboardPunct], trail[1][keyboardOther] = keyboardExtensionSpace, keyboardExtensionPunct, keyboardExtensionOther
	return pair, trail
}()

// keyboardASCII is the key code of an ASCII character of a folded name: the
// letter, a space, a punctuation mark, or keyboardOther for a digit.
var keyboardASCII = func() (out [128]uint8) {
	for c := range out {
		switch {
		case c >= 'a' && c <= 'z':
			out[c] = uint8(c - 'a')
		case c == ' ':
			out[c] = keyboardSpace
		case c >= '0' && c <= '9' || c >= 'A' && c <= 'Z':
			out[c] = keyboardOther
		default:
			out[c] = keyboardPunct
		}
	}
	return out
}()

// keyboardCode is the key code of any character: an accented letter has its
// base letter's code, a letter that is not on the keyboard never substitutes.
func keyboardCode(r rune) uint8 {
	if r < 0x80 {
		return keyboardASCII[r]
	}
	if base := []rune(stripAccents(string(r))); len(base) > 0 && isASCIILetter(base[0]) {
		return uint8(base[0] - 'a')
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return keyboardOther
	}
	if unicode.IsSpace(r) {
		return keyboardSpace
	}
	return keyboardPunct
}

// keyboardDistance is the Keyboard Distance score of two folded last names.
// ok is false when a character that never substitutes differs.
func keyboardDistance(a, b string) (float64, bool) {
	if len(a) > len(b) {
		a, b = b, a
	}
	for i := 0; i < len(b); i++ {
		if b[i] >= 0x80 || i < len(a) && a[i] >= 0x80 {
			return keyboardDistanceRunes([]rune(a), []rune(b))
		}
	}
	n := len(b)
	if n == 0 {
		return 100, true
	}
	cost, different := 0.0, false
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			different = true
			final := 0
			if i == n-1 {
				final = 1
			}
			cost += keyboardPair[final][keyboardASCII[a[i]]][keyboardASCII[b[i]]]
		}
	}
	trail := &keyboardTrail[0]
	if !different {
		trail = &keyboardTrail[1]
	}
	for i := len(a); i < n; i++ {
		code := keyboardASCII[b[i]]
		if code == keyboardOther && n-len(a) > 2 {
			return 0, false
		}
		cost += trail[code]
	}
	if cost >= keyboardNever {
		return 0, false
	}
	return 100 * (1 - cost/float64(n)), true
}

func keyboardDistanceRunes(a, b []rune) (float64, bool) {
	if len(a) > len(b) {
		a, b = b, a
	}
	n := len(b)
	cost, different := 0.0, false
	for i := range a {
		if a[i] != b[i] {
			different = true
			final := 0
			if i == n-1 {
				final = 1
			}
			cost += keyboardPair[final][keyboardCode(a[i])][keyboardCode(b[i])]
		}
	}
	trail := &keyboardTrail[0]
	if !different {
		trail = &keyboardTrail[1]
	}
	for _, r := range b[len(a):] {
		code := keyboardCode(r)
		if code == keyboardOther && n-len(a) > 2 {
			return 0, false
		}
		cost += trail[code]
	}
	if cost >= keyboardNever {
		return 0, false
	}
	return 100 * (1 - cost/float64(n)), true
}
