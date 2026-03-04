package controller

import "testing"

func TestComputeNodeDisruptionBudgetDisruptionsAllowed(t *testing.T) {
	tests := []struct {
		name                string
		maxDisruptedNodes   int
		minUndisruptedNodes int
		watchedNodes        int
		currentDisruptions  int
		expected            int
	}{
		{
			name:                "at max disruption returns zero",
			maxDisruptedNodes:   1,
			minUndisruptedNodes: 0,
			watchedNodes:        34,
			currentDisruptions:  1,
			expected:            0,
		},
		{
			name:                "max side is limiting factor",
			maxDisruptedNodes:   3,
			minUndisruptedNodes: 0,
			watchedNodes:        10,
			currentDisruptions:  2,
			expected:            1,
		},
		{
			name:                "min undisrupted side is limiting factor",
			maxDisruptedNodes:   10,
			minUndisruptedNodes: 10,
			watchedNodes:        12,
			currentDisruptions:  1,
			expected:            1,
		},
		{
			name:                "returns negative when min undisrupted cannot be respected",
			maxDisruptedNodes:   10,
			minUndisruptedNodes: 10,
			watchedNodes:        5,
			currentDisruptions:  1,
			expected:            -6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeNodeDisruptionBudgetDisruptionsAllowed(
				tt.maxDisruptedNodes,
				tt.minUndisruptedNodes,
				tt.watchedNodes,
				tt.currentDisruptions,
			)
			if got != tt.expected {
				t.Fatalf("unexpected disruptions allowed: got=%d expected=%d", got, tt.expected)
			}
		})
	}
}
