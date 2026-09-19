package domain

import "strings"

func IsPermutation(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]struct{}, len(a))
	for _, v := range a {
		aMap[strings.ToLower(v)] = struct{}{}
	}

	seen := make(map[string]struct{}, len(a))
	for _, v := range b {
		id := strings.ToLower(v)
		if _, ok := aMap[id]; !ok {
			return false
		}
		if _, dup := seen[id]; dup {
			return false
		}
		seen[id] = struct{}{}
	}

	return true
}
