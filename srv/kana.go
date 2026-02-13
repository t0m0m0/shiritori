package srv

import (
	"strings"
	"unicode/utf8"
)

// smallToNormalKana maps small kana to their normal-sized equivalents.
var smallToNormalKana = map[rune]rune{
	'ゃ': 'や', 'ゅ': 'ゆ', 'ょ': 'よ',
	'ぁ': 'あ', 'ぃ': 'い', 'ぅ': 'う', 'ぇ': 'え', 'ぉ': 'お',
	'っ': 'つ',
	'ゎ': 'わ',
}

// vowelForHiragana returns the vowel row character for a given hiragana.
// Used to resolve ー (long vowel mark).
var vowelForHiragana = map[rune]rune{
	// a-row
	'あ': 'あ', 'か': 'あ', 'さ': 'あ', 'た': 'あ', 'な': 'あ',
	'は': 'あ', 'ま': 'あ', 'や': 'あ', 'ら': 'あ', 'わ': 'あ',
	'が': 'あ', 'ざ': 'あ', 'だ': 'あ', 'ば': 'あ', 'ぱ': 'あ',
	// i-row
	'い': 'い', 'き': 'い', 'し': 'い', 'ち': 'い', 'に': 'い',
	'ひ': 'い', 'み': 'い', 'り': 'い', 'ゐ': 'い',
	'ぎ': 'い', 'じ': 'い', 'ぢ': 'い', 'び': 'い', 'ぴ': 'い',
	// u-row
	'う': 'う', 'く': 'う', 'す': 'う', 'つ': 'う', 'ぬ': 'う',
	'ふ': 'う', 'む': 'う', 'ゆ': 'う', 'る': 'う',
	'ぐ': 'う', 'ず': 'う', 'づ': 'う', 'ぶ': 'う', 'ぷ': 'う',
	// e-row
	'え': 'え', 'け': 'え', 'せ': 'え', 'て': 'え', 'ね': 'え',
	'へ': 'え', 'め': 'え', 'れ': 'え', 'ゑ': 'え',
	'げ': 'え', 'ぜ': 'え', 'で': 'え', 'べ': 'え', 'ぺ': 'え',
	// o-row
	'お': 'お', 'こ': 'お', 'そ': 'お', 'と': 'お', 'の': 'お',
	'ほ': 'お', 'も': 'お', 'よ': 'お', 'ろ': 'お', 'を': 'お',
	'ご': 'お', 'ぞ': 'お', 'ど': 'お', 'ぼ': 'お', 'ぽ': 'お',
	// n
	'ん': 'ん',
}

// katakanaToHiragana converts a single katakana rune to hiragana.
// If the rune is not katakana, it is returned unchanged.
func katakanaToHiragana(r rune) rune {
	// Katakana range: 0x30A0-0x30FF, Hiragana: 0x3040-0x309F
	if r >= 0x30A1 && r <= 0x30F6 {
		return r - 0x60
	}
	// Small katakana
	if r >= 0x30A1 && r <= 0x30FA {
		return r - 0x60
	}
	return r
}

// toHiragana converts an entire string from katakana to hiragana.
// Non-katakana characters are left unchanged.
func toHiragana(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		b.WriteRune(katakanaToHiragana(r))
	}
	return b.String()
}

// isHiragana checks if a rune is hiragana (including ん).
func isHiragana(r rune) bool {
	return r >= 0x3040 && r <= 0x309F
}

// isKatakana checks if a rune is katakana.
func isKatakana(r rune) bool {
	return r >= 0x30A0 && r <= 0x30FF
}

// isKanji checks if a rune is a CJK Unified Ideograph (common kanji range).
func isKanji(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// isLongVowelMark checks if a rune is ー.
func isLongVowelMark(r rune) bool {
	return r == 'ー'
}

// isJapanese returns true if the string contains only hiragana, katakana, or long vowel marks.
// For our shiritori game, we require words to be in kana only (no kanji).
func isJapanese(s string) bool {
	if utf8.RuneCountInString(s) == 0 {
		return false
	}
	for _, r := range s {
		if !isHiragana(r) && !isKatakana(r) && !isLongVowelMark(r) {
			return false
		}
	}
	return true
}

// normalizeSmallKana converts small kana to normal-sized kana.
func normalizeSmallKana(r rune) rune {
	if n, ok := smallToNormalKana[r]; ok {
		return n
	}
	return r
}

// getLastChar returns the last meaningful hiragana character of a word.
// It handles: ー (long vowel), small kana normalization.
// The input should already be in hiragana.
func getLastChar(hiragana string) rune {
	runes := []rune(hiragana)
	if len(runes) == 0 {
		return 0
	}

	// Walk backwards to find a meaningful last character
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]

		// If long vowel mark, resolve it from the preceding character
		if isLongVowelMark(r) {
			if i > 0 {
				prev := runes[i-1]
				prev = normalizeSmallKana(prev)
				if v, ok := vowelForHiragana[prev]; ok {
					return v
				}
			}
			continue
		}

		// Normalize small kana
		return normalizeSmallKana(r)
	}

	// Fallback
	return normalizeSmallKana(runes[len(runes)-1])
}

// getFirstChar returns the first hiragana character of a word.
// Input should already be in hiragana.
func getFirstChar(hiragana string) rune {
	for _, r := range hiragana {
		return normalizeSmallKana(r)
	}
	return 0
}

// charCount returns the number of runes in a string.
func charCount(s string) int {
	return utf8.RuneCountInString(s)
}
