package text

import "strings"

// LevenshteinDistance calculates the minimum number of single-character edits
// (insertions, deletions, or substitutions) needed to change one string into another.
func LevenshteinDistance(s1, s2 string) int {
	// Convert to lowercase for case-insensitive comparison
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)

	// If either string is empty, distance is the length of the other
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create a 2D slice to store distances
	// distances[i][j] represents the distance between s1[0:i] and s2[0:j]
	distances := make([][]int, len(s1)+1)
	for i := range distances {
		distances[i] = make([]int, len(s2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		distances[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		distances[0][j] = j
	}

	// Fill in the rest of the matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			distances[i][j] = minOfThree(
				distances[i-1][j]+1,      // deletion
				distances[i][j-1]+1,      // insertion
				distances[i-1][j-1]+cost, // substitution
			)
		}
	}

	return distances[len(s1)][len(s2)]
}

// FindSimilarStrings returns strings from candidates that are within maxDistance
// edits from the query string. Results are sorted by distance (closest first).
func FindSimilarStrings(query string, candidates []string, maxDistance int) []string {
	type suggestion struct {
		str      string
		distance int
	}

	var suggestions []suggestion

	for _, candidate := range candidates {
		distance := LevenshteinDistance(query, candidate)
		
		// Only include if within threshold and not exact match
		if distance > 0 && distance <= maxDistance {
			suggestions = append(suggestions, suggestion{
				str:      candidate,
				distance: distance,
			})
		}
	}

	// Sort by distance (closest first)
	for i := 0; i < len(suggestions); i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].distance < suggestions[i].distance {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	// Extract just the strings
	var result []string
	for _, s := range suggestions {
		result = append(result, s.str)
	}

	return result
}

// minOfThree returns the minimum of three integers
func minOfThree(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}