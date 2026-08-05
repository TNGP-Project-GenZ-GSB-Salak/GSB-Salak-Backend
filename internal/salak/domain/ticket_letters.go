package domain

import "fmt"

// thaiConsonantStart/End span the Thai consonant block (U+0E01 ก ... U+0E2E
// ฮ). Two code points inside that range, ฤ (U+0E24) and ฦ (U+0E26), are
// independent vowels rather than consonants and are excluded from the
// ticket-letter cycle - see docs/GAPS.md §2.3.
const (
	thaiConsonantStart = 0x0E01
	thaiConsonantEnd   = 0x0E2E
	thaiVowelRu        = 0x0E24 // ฤ
	thaiVowelLu        = 0x0E26 // ฦ
)

// ticketLetters is the ordered cycle a product's ticket_sequence cursor
// advances through: the 44 true Thai consonants, in Unicode order, with ฤ
// and ฦ skipped. Built from the code-point range rather than hand-
// transcribed, so the exclusion is exact and self-documenting.
var ticketLetters = buildTicketLetters()

func buildTicketLetters() []rune {
	letters := make([]rune, 0, 44)
	for cp := thaiConsonantStart; cp <= thaiConsonantEnd; cp++ {
		if cp == thaiVowelRu || cp == thaiVowelLu {
			continue
		}
		letters = append(letters, rune(cp))
	}
	return letters
}

// NextLetter returns the ticket letter that follows current in the
// skip-aware cycle (e.g. ร -> ล, skipping ฤ) - advancing is a skip, not a
// naive +1 offset, precisely because ฤ/ฦ are excluded. Returns an error if
// current isn't a single rune in the cycle, or if current is already the
// last letter (ฮ) - a product exhausting all 44 letters (440,000,000
// tickets) is not an expected path, so this is a defensive guard rather
// than a real rollover case.
func NextLetter(current string) (string, error) {
	runes := []rune(current)
	if len(runes) != 1 {
		return "", fmt.Errorf("invalid ticket letter %q: must be exactly one rune", current)
	}

	idx := -1
	for i, r := range ticketLetters {
		if r == runes[0] {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "", fmt.Errorf("invalid ticket letter %q: not in the 44-consonant cycle", current)
	}
	if idx == len(ticketLetters)-1 {
		return "", fmt.Errorf("ticket letter cycle exhausted: %q is the last of %d letters", current, len(ticketLetters))
	}
	return string(ticketLetters[idx+1]), nil
}
