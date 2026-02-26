package text

import (
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{
			name:     "identical strings",
			s1:       "pod-scenarios",
			s2:       "pod-scenarios",
			expected: 0,
		},
		{
			name:     "one character difference",
			s1:       "pod-scenario",
			s2:       "pod-scenarios",
			expected: 1,
		},
		{
			name:     "case insensitive",
			s1:       "Pod-Scenarios",
			s2:       "pod-scenarios",
			expected: 0,
		},
		{
			name:     "completely different",
			s1:       "pod",
			s2:       "network",
			expected: 6, // Fixed: actual distance is 6, not 7
		},
		{
			name:     "empty string",
			s1:       "",
			s2:       "test",
			expected: 4,
		},
		{
			name:     "both empty",
			s1:       "",
			s2:       "",
			expected: 0,
		},
		{
			name:     "multiple edits",
			s1:       "pod-scenari",
			s2:       "pod-scenarios",
			expected: 2,
		},
		{
			name:     "typo in middle",
			s1:       "nod-scenarios",
			s2:       "node-scenarios",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LevenshteinDistance(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d, want %d",
					tt.s1, tt.s2, result, tt.expected)
			}
		})
	}
}

func TestFindSimilarStrings(t *testing.T) {
	candidates := []string{
		"pod-scenarios",
		"node-scenarios",
		"network-chaos",
		"zone-outage",
		"container-scenarios",
	}

	tests := []struct {
		name        string
		query       string
		maxDistance int
		wantCount   int        // Just check count, not exact order
		shouldContain []string  // Check these are present
	}{
		{
			name:        "close match - one typo",
			query:       "pod-scenario",
			maxDistance: 3,
			wantCount:   2, // pod-scenarios (dist=1) and node-scenarios (dist=3)
			shouldContain: []string{"pod-scenarios"},
		},
		{
			name:        "multiple close matches",
			query:       "nod-scenarios",
			maxDistance: 3,
			wantCount:   2,
			shouldContain: []string{"node-scenarios", "pod-scenarios"},
		},
		{
			name:        "no matches within threshold",
			query:       "xyz",
			maxDistance: 3,
			wantCount:   0,
			shouldContain: []string{},
		},
		{
			name:        "exact match excluded",
			query:       "pod-scenarios",
			maxDistance: 3,
			wantCount:   1, // node-scenarios is distance 3
			shouldContain: []string{},
		},
		{
			name:        "wider threshold catches more",
			query:       "pod",
			maxDistance: 10,
			wantCount:   2, // pod-scenarios and zone-outage
			shouldContain: []string{"pod-scenarios"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindSimilarStrings(tt.query, candidates, tt.maxDistance)
			
			// Check count
			if len(result) != tt.wantCount {
				t.Errorf("FindSimilarStrings() returned %d results, want %d\nGot: %v",
					len(result), tt.wantCount, result)
			}

			// Check that expected strings are present
			for _, expected := range tt.shouldContain {
				found := false
				for _, r := range result {
					if r == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("FindSimilarStrings() missing expected result %q\nGot: %v",
						expected, result)
				}
			}
		})
	}
}

func TestFindSimilarStrings_Sorting(t *testing.T) {
	// Test that results are sorted by distance
	candidates := []string{
		"pod-scenarios",    // distance 2 from "pod-scenario"
		"node-scenarios",   // distance 3 from "pod-scenario"  
		"container-scenarios", // distance > 10
	}

	result := FindSimilarStrings("pod-scenari", candidates, 5)
	
	// Should return both, with pod-scenarios first (closer match)
	if len(result) < 2 {
		t.Errorf("Expected at least 2 results, got %d: %v", len(result), result)
		return
	}

	// First result should be the closest match
	if result[0] != "pod-scenarios" {
		t.Errorf("Expected first result to be 'pod-scenarios' (closest), got %q", result[0])
	}
}