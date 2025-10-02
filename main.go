package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const cacheFileName = "dict_cache.json"

type SubDict struct {
	Example string `json:"example"`
	SubPos string `json:"sub_pos"`
	Def string `json:"definition"`
	Pos string `json:"pos"`
}

type Dictionary struct {
	Word string
	SubDict []SubDict
}

type SyllableCond struct {
	Length int
}

func loadDictFromURL(url string) (map[string][]SubDict, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch: status %d", resp.StatusCode)
	}

	var dict map[string][]SubDict
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&dict)
	if err != nil {
		return nil, err
	}
	return dict, nil
}

func loadDict(url string) (map[string][]SubDict, error) {
	cachePath := os.TempDir() + string(os.PathSeparator) + cacheFileName

	if _, err := os.Stat(cachePath); err == nil {
		data, err := os.ReadFile(cachePath)
		if err == nil {
			var dict map[string][]SubDict
			if err := json.Unmarshal(data, &dict); err == nil {
				return dict, nil
			}
		}
	}

	dict, err := loadDictFromURL(url)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(dict)
	_ = os.WriteFile(cachePath, data, 0644)

	return dict, nil
}

func parsePattern(query string) ([]SyllableCond, string) {
	parts := strings.Split(query, "-")
	conds := []SyllableCond{}
	mustChars := ""

	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			conds = append(conds, SyllableCond{Length: n})
		} else {
			mustChars += p
		}
	}
	return conds, mustChars
}

func removeDiacritics(str string) string {
	t := norm.NFD.String(str)

	var b strings.Builder
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		switch r {
			case 'đ':
				b.WriteRune('d')
			case 'Đ':
				b.WriteRune('D')
			default:
				b.WriteRune(r)
		}
	}
	return b.String()
}

func matchWildcard(pattern, text string) bool {
	pRunes := []rune(pattern)
	tRunes := []rune(text)

	if len(pRunes) != len(tRunes) {
		return false
	}

	for i := range pRunes {
		if pRunes[i] == '*' {
			continue
		}
		if pRunes[i] != tRunes[i] {
			return false
		}
	}
	return true
}

func matchSyllablePatternWithChars(word string, conds []SyllableCond, mustChars string) bool {
	parts := strings.Fields(word)
	if len(parts) != len(conds) {
		return false
	}

	for i, part := range parts {
		if utf8.RuneCountInString(part) != conds[i].Length {
			return false
		}
	}

	for _, ch := range mustChars {
		if !strings.ContainsRune(word, ch) {
			return false
		}
	}
	return true
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func main() {
	clearScreen()
	fmt.Println("Loading dictionary...")
	url := "https://raw.githubusercontent.com/minhqnd/wordle-vietnamese/main/lib/dictionary_vi.json"
	dict, _ := loadDict(url)

	usage := `
Hướng dẫn sử dụng
Tất cả các ký tự được nhập phải là chữ cái trong bảng chữ cái tiếng Anh.
So khớp từ cụ thể: Sử dụng * cho các chữ cái bất kỳ. Phân tách các âm tiết bằng dấu cách.
Ví dụ:
- "viet ***" sẽ khớp với các từ "viet nam", "viet hoa", "viet ngu"
- "*an" sẽ khớp với các từ "can", "san", "tan", "man", v.v.
Âm tiết và số lượng từ trong âm tiết: Sử dụng số nguyên cho số lượng chữ cái trong âm tiết và dấu gạch nối để phân tách các âm tiết. Bạn có thể thêm các chữ cái vào cuối để khớp với các từ chứa các chữ cái đó.
Ví dụ:
- "3-4" sẽ khớp với các từ có 7 chữ cái và 2 âm tiết, trong đó âm tiết thứ nhất có 3 chữ cái, âm tiết thứ hai có 4 chữ cái.
- "3-4-3-aug" sẽ khớp với các từ có 7 chữ cái và 3 âm tiết, trong đó âm tiết thứ nhất có 3 chữ cái, âm tiết thứ hai có 4 chữ cái, âm tiết thứ ba có 3 chữ cái và từ phải chứa tất cả các chữ cái "a", "u", "g" bất kể số lần xuất hiện, thứ tự và vị trí.

Usage:
All characters entered must be letters of the English alphabet
Specific word matching: Use * for arbitrary letters. Separate syllables with spaces.

For example:
- "viet ***" will match the words "viet tay", "viet hoa", "viet nam", etc
- "*an" will match the words "can", "san", "tan", "man", etc

Syllable and number of words in syllable: Use integers for the number of letters in the syllable and hyphens to separate syllables. You can add letters at the end to match words containing those letters.
For example:
- "3-4-3" will match words with 7 letters and 3 syllables, the first syllable has 3 letters, the second syllable has 4 letters, the third syllable has 3 letters
- "3-4-aug" will match words with 7 letters and 2 syllables, the first syllable has 3 letters, the second syllable has 4 letters and the word must contain all the letters "a", "u", "g" regardless of the number of occurrences, order and position
`

	fmt.Println("Type !help for usage")
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(">>> ")
	query, _ := reader.ReadString('\n')
	query = strings.TrimSpace(query)

	switch {
		case query == "!help":
			fmt.Println(usage)

		case strings.Contains(query, "-"):
			conds, mustChars := parsePattern(query)
			for word := range dict {
				if matchSyllablePatternWithChars(removeDiacritics(word), conds, mustChars) {
					fmt.Println(">", word)
				}
			}
		default:
			queryNorm := strings.ToLower(removeDiacritics(query))
			for word := range dict {
				wordNorm := strings.ToLower(removeDiacritics(word))
				if matchWildcard(queryNorm, wordNorm) {
					fmt.Printf("> %s\n", word)
				}
			}
	}
}
