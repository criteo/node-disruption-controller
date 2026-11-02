package v1alpha1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNoticePeriodFor(t *testing.T) {
	spec := ApplicationDisruptionBudgetSpec{
		NoticePeriod: &metav1.Duration{Duration: 30 * time.Minute},
		NoticePeriodOverrides: []TypedNoticePeriod{
			{Type: "tor-maintenance", NoticePeriod: metav1.Duration{Duration: 10 * time.Minute}},
		},
	}
	if got := spec.NoticePeriodFor("tor-maintenance"); got != 10*time.Minute {
		t.Errorf("override should win, got %v", got)
	}
	if got := spec.NoticePeriodFor("maintenance"); got != 30*time.Minute {
		t.Errorf("scalar fallback, got %v", got)
	}
	empty := ApplicationDisruptionBudgetSpec{}
	if got := empty.NoticePeriodFor("maintenance"); got != 0 {
		t.Errorf("absent means zero, got %v", got)
	}
}
