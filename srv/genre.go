package srv

import (
	"bufio"
	_ "embed"
	"strings"
)

//go:embed wordlists/food.txt
var foodTxt string

//go:embed wordlists/animal.txt
var animalTxt string

// genreWords contains word lists for each genre.
var genreWords map[string]map[string]bool

func init() {
	genreWords = map[string]map[string]bool{
		"食べ物": loadWordSet(foodTxt),
		"動物":  loadWordSet(animalTxt),
	}
}

// loadWordSet reads a newline-delimited string into a set.
func loadWordSet(data string) map[string]bool {
	m := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		w := strings.TrimSpace(scanner.Text())
		if w != "" {
			m[w] = true
		}
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
