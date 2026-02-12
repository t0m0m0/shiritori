package srv

// genreWords contains word lists for each genre.
// All words are in hiragana.
var genreWords = map[string]map[string]bool{
	"食べ物": toSet([]string{
		"りんご", "すいか", "たまご", "みかん", "ばなな",
		"いちご", "めろん", "ぶどう", "もも", "なし",
		"かき", "きうい", "れもん", "おれんじ", "さくらんぼ",
		"とまと", "きゅうり", "なす", "にんじん", "たまねぎ",
		"じゃがいも", "きゃべつ", "れたす", "ぴーまん", "ほうれんそう",
		"だいこん", "ごぼう", "ねぎ", "にんにく", "しょうが",
		"おにぎり", "すし", "らーめん", "うどん", "そば",
		"てんぷら", "やきにく", "からあげ", "ぎょうざ", "おでん",
		"みそしる", "たこやき", "おこのみやき", "やきそば", "かれー",
		"ぱん", "けーき", "くっきー", "ちょこれーと", "あいす",
		"ぷりん", "だんご", "もち", "せんべい", "ようかん",
		"まぐろ", "さけ", "さば", "いわし", "たい",
		"えび", "かに", "いか", "たこ", "あさり",
		"とうふ", "なっとう", "みそ", "しょうゆ", "わさび",
		"こめ", "むぎ", "そうめん", "うめぼし", "つけもの",
		"にく", "ぶたにく", "とりにく", "ぎゅうにく", "はむ",
		"そーせーじ", "べーこん", "ちーず", "ばたー", "みるく",
		"こーひー", "こうちゃ", "じゅーす", "みず", "おちゃ",
	}),
	"動物": toSet([]string{
		"いぬ", "ねこ", "うま", "うし", "ぶた",
		"ひつじ", "やぎ", "にわとり", "あひる", "うさぎ",
		"ねずみ", "はむすたー", "りす", "きつね", "たぬき",
		"おおかみ", "くま", "しか", "さる", "ごりら",
		"ぞう", "きりん", "しまうま", "かば", "さい",
		"らいおん", "とら", "ひょう", "ちーたー", "ぱんだ",
		"こあら", "かんがるー", "いるか", "くじら", "あざらし",
		"おっとせい", "ぺんぎん", "ふらみんご", "つる", "たか",
		"わし", "ふくろう", "すずめ", "からす", "はと",
		"いんこ", "おうむ", "かめ", "とかげ", "へび",
		"わに", "かえる", "さかな", "くらげ", "ひとで",
		"かぶとむし", "くわがた", "ちょう", "とんぼ", "せみ",
		"ばった", "かまきり", "てんとうむし", "ほたる", "あり",
		"はち", "かたつむり", "だちょう", "あるぱか", "らくだ",
		"はりねずみ", "もぐら", "いたち", "かわうそ", "びーばー",
		"ひよこ", "こいぬ", "こねこ", "こうま", "こじか",
	}),
}

// toSet converts a slice of strings into a set (map[string]bool).
func toSet(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// isWordInGenre checks if a word (in hiragana) belongs to the specified genre.
// If genre is empty or "なし", any word is accepted.
func isWordInGenre(hiraganaWord, genre string) bool {
	if genre == "" || genre == "なし" {
		return true
	}
	words, ok := genreWords[genre]
	if !ok {
		// Unknown genre: accept any word
		return true
	}
	return words[hiraganaWord]
}

// getGenreList returns available genre names.
func getGenreList() []string {
	genres := []string{"なし"}
	for k := range genreWords {
		genres = append(genres, k)
	}
	return genres
}
