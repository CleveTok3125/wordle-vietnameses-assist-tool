package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"unicode"

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

func matchWildcard(pattern, text, required, excluded string) bool {
	pattern = normalizeSpaces(pattern)

	pWords := strings.Split(pattern, " ")
	tWords := strings.Split(text, " ")

	if len(pWords) != len(tWords) {
		return false
	}

	var allWildcardRunes []rune

	for i := range pWords {
		pw := []rune(pWords[i])
		tw := []rune(tWords[i])

		if len(pw) != len(tw) {
			return false
		}

		for j := range pw {
			if pw[j] == '*' {
				allWildcardRunes = append(allWildcardRunes, tw[j])
			} else {
				if pw[j] != tw[j] {
					return false
				}
			}
		}
	}

	wildcardText := string(allWildcardRunes)

	for _, ch := range required {
		if !strings.ContainsRune(wildcardText, ch) {
			return false
		}
	}
	for _, ch := range excluded {
		if strings.ContainsRune(wildcardText, ch) {
			return false
		}
	}

	return true
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func ui(dict map[string][]SubDict) {
	usage := `
Hướng dẫn sử dụng
Tất cả các ký tự được nhập phải là chữ cái trong bảng chữ cái tiếng Anh.

So khớp từ cụ thể: Sử dụng * cho các chữ cái bất kỳ. Phân tách các âm tiết bằng dấu cách. Bạn có thể thêm các chữ cái vào cuối để khớp với các từ không chứa (-) hoặc chứa (+) các chữ cái đó.
Ví dụ:
- "viet ***" sẽ khớp với các từ "viet nam", "viet hoa", "viet ngu", v.v.
- "*an" sẽ khớp với các từ "can", "san", "tan", "man", v.v.
- "viet *** -oh" sẽ khớp với các từ "viet nam", "viet ngu", "viet tay" nhưng không khớp "viet hoa" vì từ đó có chứa "o" và "h".
- "viet *** +a" sẽ khớp với các từ "viet nam", "viet tay" nhưng không khớp "viet ngu" vì từ đó không chứa "a".
- "viet *** -oh +a" hoặc "viet *** +a -oh" sẽ kết hợp điều kiện từ hai ví dụ trên.

Mẹo: sử dụng khớp bao gồm với các chữ cái có trong từ nhưng sai vị trí và khớp loại trừ với các chữ cái không có trong từ từ các lần đoán trước.

Usage:
All characters entered must be letters of the English alphabet.

Match specific words: Use * for any letters. Separate syllables with spaces. You can add letters at the end to match words that do not contain (-) or contain (+) those letters.
For example:
- "viet ***" will match the words "viet nam", "viet hoa", "viet ngu", etc.
- "*an" will match the words "can", "san", "tan", "man", etc.
- "viet *** -oh" will match the words "viet nam", "viet ngu", "viet tay" but not "viet hoa" because it contains "o" and "h".
- "viet *** +a" will match the words "viet nam", "viet tay" but not "viet ngu" because it does not contain "a".
- "viet *** -oh +a" or "viet *** +a -oh" will combine the conditions from the two examples above.

Tip: Use inclusive matches for letters that are in the word but in the wrong position, and exclusive matches for letters that are not in the word from previous guesses.
`

	reader := bufio.NewReader(os.Stdin)
	fmt.Print(">>> ")
	query, _ := reader.ReadString('\n')
	query = strings.TrimSpace(query)

	switch  query{
		case "!quit":
			os.Exit(0)
		case "!help":
			fmt.Println(usage)
			os.Exit(0)
		case "!clear":
			clearScreen()
		default:
			queryNorm := strings.ToLower(removeDiacritics(query))

			queryParts := strings.Split(queryNorm, " ")
			queryPattern := ""
			excludedChars := ""
			requiredChars := ""

			for _, part := range queryParts {
				if len(part) > 0 && part[0] == '-' {
					excludedChars += part[1:]
				} else if len(part) > 0 && part[0] == '+' {
					requiredChars += part[1:]
				} else {
					if queryPattern != "" {
						queryPattern += " "
					}
					queryPattern += part
				}
			}

			// fmt.Println(queryPattern)
			// fmt.Println(excludedChars)
			// fmt.Println(requiredChars)

			for word := range dict {
				wordNorm := normalizeSpaces(word)
				wordNorm = strings.ToLower(removeDiacritics(wordNorm))
				if matchWildcard(queryPattern, wordNorm, requiredChars, excludedChars) {
					fmt.Printf("> %s\n", word)
				}
			}
	}
}

func main() {
	clearScreen()
	fmt.Println("Loading dictionary...")
	url := "https://raw.githubusercontent.com/minhqnd/wordle-vietnamese/main/lib/dictionary_vi.json"
	dict, _ := loadDict(url)
	fmt.Println("Type:\n- !help for usage\n- !quit for quit\n- !clear for clear screen")
	for {
		ui(dict)
	}
}
