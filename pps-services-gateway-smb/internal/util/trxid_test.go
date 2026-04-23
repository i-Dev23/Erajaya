package util

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateTransactionID(t *testing.T) {
	ts := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		mid      string
		msgID    string
		contains []string
	}{
		{"normal", "MID001", "12345", []string{"SMB-", "MID001", "12345", "20260422"}},
		{"empty mid", "", "12345", []string{"SMB-", "UNKNOWN", "12345"}},
		{"empty msgID", "MID001", "", []string{"SMB-", "MID001", "0"}},
		{"both empty", "", "", []string{"SMB-", "UNKNOWN", "0"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTransactionID(tt.mid, tt.msgID, ts)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("GenerateTransactionID(%q, %q) = %q, missing %q", tt.mid, tt.msgID, result, substr)
				}
			}
		})
	}
}

func TestGenerateTransactionID_Uniqueness(t *testing.T) {
	t1 := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 22, 14, 30, 1, 0, time.UTC)

	id1 := GenerateTransactionID("MID", "1", t1)
	id2 := GenerateTransactionID("MID", "1", t2)

	if id1 == id2 {
		t.Error("different timestamps should produce different IDs")
	}
}
