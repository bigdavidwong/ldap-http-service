package utils

import (
	"strings"
)

func isSpecialChar(char rune) bool {
	specialChars := "~`!@#$%^&*()_-+={}[]|\\:;<,>.?/"
	for _, special := range specialChars {
		if char == special {
			return true
		}
	}
	return false
}

func containsSubstring(username, password string) bool {
	username = strings.ToLower(username)
	password = strings.ToLower(password)

	for i := 0; i < len(username)-1; i++ {
		substr := username[i : i+2]
		if strings.Contains(password, substr) {
			return true
		}
	}
	return false
}
