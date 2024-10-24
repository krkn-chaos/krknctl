package text

import (
	"math/rand"
	"strings"
)

func justifyLine(words []string, width int) string {
	if len(words) == 1 {
		return words[0] + strings.Repeat(" ", width-len(words[0]))
	}
	totalSpaces := width
	for _, word := range words {
		totalSpaces -= len(word)
	}
	spacesBetweenWords := totalSpaces / (len(words) - 1)
	extraSpaces := totalSpaces % (len(words) - 1)

	var justifiedLine string
	for i, word := range words {
		justifiedLine += word
		if i < len(words)-1 {
			spaceCount := spacesBetweenWords
			if i < extraSpaces {
				spaceCount++
			}
			justifiedLine += strings.Repeat(" ", spaceCount)
		}
	}
	return justifiedLine
}

func Justify(text string, width int) []string {
	words := strings.Fields(text)
	var result []string
	var line []string
	lineLength := 0

	for _, word := range words {
		if lineLength+len(word)+len(line) > width {
			result = append(result, justifyLine(line, width))
			line = []string{}
			lineLength = 0
		}
		line = append(line, word)
		lineLength += len(word)
	}

	if len(line) > 0 {
		lastLine := strings.Join(line, " ")
		calculatedwidth := width - len(lastLine)
		if calculatedwidth < 0 {
			calculatedwidth = 0
		}
		result = append(result, lastLine+strings.Repeat(" ", calculatedwidth))
	}

	return result
}

func RandString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
