package main

import (
	"strings"
)

func filterWords(text string) string {
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(text, " ")
	cleanedWords := []string{}

	for _, word := range words {
		isBad := false
		for _, bad := range badWords {
			if strings.EqualFold(word, bad) {
				isBad = true
				break
			}
		}
		if isBad {
			cleanedWords = append(cleanedWords, "****")
		} else {
			cleanedWords = append(cleanedWords, word)
		}
	}
	return strings.Join(cleanedWords, " ")
}
