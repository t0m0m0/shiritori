package srv

import (
	"fmt"
	"net/http"
	"strings"
)

// HandleOGPImage generates an SVG OGP image for a game result.
func (s *Server) HandleOGPImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	result, err := s.loadResult(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Build word chain
	words := make([]string, len(result.History))
	for i, h := range result.History {
		words[i] = h.Word
	}

	// Title
	title := fmt.Sprintf("%d語のしりとり！", len(result.History))
	if result.Winner != "" {
		title = fmt.Sprintf("%sさんの勝利！（%d語）", result.Winner, len(result.History))
	}

	// Build chain lines (wrap at ~18 chars per line, max 4 lines)
	chainLines := wrapChain(words, 16, 4)

	// Build scores
	type scoreEntry struct {
		Name  string
		Score int
		Medal string
	}
	var scores []scoreEntry
	// Sort by score desc
	type kv struct {
		K string
		V int
	}
	var sorted []kv
	for k, v := range result.Scores {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].V > sorted[i].V {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	medals := []string{"🥇", "🥈", "🥉"}
	for i, kv := range sorted {
		m := ""
		if i < len(medals) {
			m = medals[i]
		}
		scores = append(scores, scoreEntry{kv.K, kv.V, m})
	}

	// Genre tag
	genreTag := ""
	if result.Genre != "" && result.Genre != "なし" {
		genreTag = fmt.Sprintf(`<text x="600" y="68" text-anchor="end" font-size="18" fill="#818cf8" font-weight="500">ジャンル: %s</text>`, svgEsc(result.Genre))
	}

	// Build score rows SVG
	scoreRows := ""
	for i, sc := range scores {
		if i >= 4 {
			break
		}
		y := 130 + i*36
		bg := "#f1f0fb"
		if i == 0 {
			bg = "#fef3c7"
		}
		scoreRows += fmt.Sprintf(
			`<rect x="40" y="%d" width="250" height="30" rx="6" fill="%s"/>`+
				`<text x="56" y="%d" font-size="16">%s</text>`+
				`<text x="82" y="%d" font-size="15" font-weight="600" fill="#1e1b4b">%s</text>`+
				`<text x="270" y="%d" text-anchor="end" font-size="15" font-weight="700" fill="#6366f1">%d点</text>`,
			y, bg,
			y+21, sc.Medal,
			y+21, svgEsc(sc.Name),
			y+21, sc.Score,
		)
	}

	// Build chain SVG
	chainSVG := ""
	for i, line := range chainLines {
		y := 145 + i*28
		chainSVG += fmt.Sprintf(
			`<text x="480" y="%d" text-anchor="middle" font-size="15" fill="#4f46e5" font-weight="500">%s</text>`,
			y, svgEsc(line),
		)
	}

	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 640 330">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%%" stop-color="#6366f1"/>
      <stop offset="100%%" stop-color="#818cf8"/>
    </linearGradient>
  </defs>
  <rect width="640" height="330" rx="0" fill="url(#bg)"/>
  <rect x="16" y="16" width="608" height="298" rx="16" fill="white" opacity="0.97"/>
  
  <!-- Title -->
  <text x="320" y="60" text-anchor="middle" font-size="24" font-weight="900" fill="#1e1b4b" font-family="sans-serif">🎌 %s</text>
  %s

  <!-- Divider -->
  <line x1="320" y1="90" x2="320" y2="290" stroke="#e5e7eb" stroke-width="1" stroke-dasharray="4,4"/>

  <!-- Scores -->
  <text x="165" y="118" text-anchor="middle" font-size="14" font-weight="700" fill="#6b7280" font-family="sans-serif">スコア</text>
  %s

  <!-- Chain -->
  <text x="480" y="118" text-anchor="middle" font-size="14" font-weight="700" fill="#6b7280" font-family="sans-serif">しりとりチェーン</text>
  %s

  <!-- Footer -->
  <text x="320" y="310" text-anchor="middle" font-size="12" fill="#a5b4fc" font-family="sans-serif">しりとり - マルチプレイヤー</text>
</svg>`,
		svgEsc(title), genreTag, scoreRows, chainSVG)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write([]byte(svg))
}

func svgEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// wrapChain breaks a word chain into lines of approximately maxRunes rune-width.
func wrapChain(words []string, maxPerLine int, maxLines int) []string {
	if len(words) == 0 {
		return []string{"（なし）"}
	}

	var lines []string
	var current []string
	currentLen := 0

	for i, w := range words {
		sep := ""
		if i > 0 {
			sep = " → "
		}
		addLen := len([]rune(sep)) + len([]rune(w))
		if currentLen > 0 && currentLen+addLen > maxPerLine {
			lines = append(lines, strings.Join(current, " → "))
			current = []string{w}
			currentLen = len([]rune(w))
		} else {
			current = append(current, w)
			currentLen += addLen
		}
	}
	if len(current) > 0 {
		lines = append(lines, strings.Join(current, " → "))
	}

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] += " …"
	}
	return lines
}
