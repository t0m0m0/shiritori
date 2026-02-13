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

// KanaRow represents a row (行) of the Japanese kana table.
type KanaRow struct {
	Name  string // e.g. "あ行"
	Label string // e.g. "あ"
	Chars []rune // all hiragana characters in this row
}

// KanaRows defines all kana rows in standard order.
// Dakuten/handakuten variants are grouped with their base row.
var KanaRows = []KanaRow{
	{Name: "あ行", Label: "あ", Chars: []rune{'あ', 'い', 'う', 'え', 'お'}},
	{Name: "か行", Label: "か", Chars: []rune{'か', 'き', 'く', 'け', 'こ', 'が', 'ぎ', 'ぐ', 'げ', 'ご'}},
	{Name: "さ行", Label: "さ", Chars: []rune{'さ', 'し', 'す', 'せ', 'そ', 'ざ', 'じ', 'ず', 'ぜ', 'ぞ'}},
	{Name: "た行", Label: "た", Chars: []rune{'た', 'ち', 'つ', 'て', 'と', 'だ', 'ぢ', 'づ', 'で', 'ど'}},
	{Name: "な行", Label: "な", Chars: []rune{'な', 'に', 'ぬ', 'ね', 'の'}},
	{Name: "は行", Label: "は", Chars: []rune{'は', 'ひ', 'ふ', 'へ', 'ほ', 'ば', 'び', 'ぶ', 'べ', 'ぼ', 'ぱ', 'ぴ', 'ぷ', 'ぺ', 'ぽ'}},
	{Name: "ま行", Label: "ま", Chars: []rune{'ま', 'み', 'む', 'め', 'も'}},
	{Name: "や行", Label: "や", Chars: []rune{'や', 'ゆ', 'よ'}},
	{Name: "ら行", Label: "ら", Chars: []rune{'ら', 'り', 'る', 'れ', 'ろ'}},
	{Name: "わ行", Label: "わ", Chars: []rune{'わ', 'を', 'ん'}},
}

// kanaRowMap maps each hiragana character to its row name.
var kanaRowMap map[rune]string

func init() {
	kanaRowMap = make(map[rune]string)
	for _, row := range KanaRows {
		for _, ch := range row.Chars {
			kanaRowMap[ch] = row.Name
		}
	}
}

// GetKanaRow returns the row name (e.g. "あ行") for a hiragana character.
// Returns "" if the character is not found.
func GetKanaRow(r rune) string {
	// Normalize small kana first
	r = normalizeSmallKana(r)
	return kanaRowMap[r]
}

// ValidateAllowedRows checks that every character in a hiragana word
// belongs to one of the allowed rows. Returns the first offending character
// and its row name, or empty strings if all characters are valid.
func ValidateAllowedRows(hiragana string, allowedRows []string) (badChar rune, badRow string) {
	if len(allowedRows) == 0 {
		return 0, ""
	}
	allowed := make(map[string]bool, len(allowedRows))
	for _, r := range allowedRows {
		allowed[r] = true
	}
	for _, r := range hiragana {
		if isLongVowelMark(r) {
			continue // skip ー
		}
		row := GetKanaRow(r)
		if row == "" {
			continue // unknown char (shouldn't happen after isJapanese check)
		}
		if !allowed[row] {
			return r, row
		}
	}
	return 0, ""
}

// GetKanaRowNames returns all row names in order.
func GetKanaRowNames() []string {
	names := make([]string, len(KanaRows))
	for i, r := range KanaRows {
		names[i] = r.Name
	}
	return names
}

// dakutenSet contains all hiragana characters with dakuten (濁点).
var dakutenSet = map[rune]bool{
	'が': true, 'ぎ': true, 'ぐ': true, 'げ': true, 'ご': true,
	'ざ': true, 'じ': true, 'ず': true, 'ぜ': true, 'ぞ': true,
	'だ': true, 'ぢ': true, 'づ': true, 'で': true, 'ど': true,
	'ば': true, 'び': true, 'ぶ': true, 'べ': true, 'ぼ': true,
}

// handakutenSet contains all hiragana characters with handakuten (半濁点).
var handakutenSet = map[rune]bool{
	'ぱ': true, 'ぴ': true, 'ぷ': true, 'ぺ': true, 'ぽ': true,
}

// IsDakuten returns true if the rune is a hiragana character with dakuten.
func IsDakuten(r rune) bool {
	return dakutenSet[r]
}

// IsHandakuten returns true if the rune is a hiragana character with handakuten.
func IsHandakuten(r rune) bool {
	return handakutenSet[r]
}

// ContainsDakutenHandakuten returns true if the hiragana string contains
// any dakuten or handakuten characters.
func ContainsDakutenHandakuten(hiragana string) bool {
	for _, r := range hiragana {
		if dakutenSet[r] || handakutenSet[r] {
			return true
		}
	}
	return false
}

// ValidateNoDakuten checks that a hiragana string contains no dakuten or
// handakuten characters. Returns the first offending rune, or 0 if valid.
func ValidateNoDakuten(hiragana string) rune {
	for _, r := range hiragana {
		if dakutenSet[r] || handakutenSet[r] {
			return r
		}
	}
	return 0
}

