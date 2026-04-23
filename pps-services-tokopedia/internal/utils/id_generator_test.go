package utils

import (
	"strings"
	"testing"
	"time"
)

func TestGeneratePPSRequestID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Generate PPS Request ID",
			want: "PPS-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GeneratePPSRequestID()

			// Check if ID starts with "PPS-"
			if !strings.HasPrefix(id, tt.want) {
				t.Errorf("GeneratePPSRequestID() = %v, want prefix %v", id, tt.want)
			}

			// Check if ID is long enough (should contain timestamp + random string)
			if len(id) < 20 {
				t.Errorf("GeneratePPSRequestID() = %v, length too short", id)
			}

			// Check if ID contains timestamp format (YYYYMMDD-HHMMSS)
			parts := strings.Split(id, "-")
			if len(parts) < 3 {
				t.Errorf("GeneratePPSRequestID() = %v, should have at least 3 parts separated by '-'", id)
			}

			// Check if second part is date (YYYYMMDD)
			if len(parts[1]) != 8 {
				t.Errorf("GeneratePPSRequestID() date part = %v, should be 8 characters", parts[1])
			}

			// Check if third part is time (HHMMSS)
			if len(parts[2]) != 6 {
				t.Errorf("GeneratePPSRequestID() time part = %v, should be 6 characters", parts[2])
			}
		})
	}
}

func TestGeneratePPSRequestIDWithPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{
			name:   "Generate with custom prefix",
			prefix: "CUSTOM",
			want:   "CUSTOM-",
		},
		{
			name:   "Generate with TEST prefix",
			prefix: "TEST",
			want:   "TEST-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GeneratePPSRequestIDWithPrefix(tt.prefix)

			// Check if ID starts with custom prefix
			if !strings.HasPrefix(id, tt.want) {
				t.Errorf("GeneratePPSRequestIDWithPrefix() = %v, want prefix %v", id, tt.want)
			}

			// Check if ID is long enough
			if len(id) < 20 {
				t.Errorf("GeneratePPSRequestIDWithPrefix() = %v, length too short", id)
			}
		})
	}
}

func TestGenerateInquiryID(t *testing.T) {
	id := GenerateInquiryID()

	// Check if ID starts with "INQ-"
	if !strings.HasPrefix(id, "INQ-") {
		t.Errorf("GenerateInquiryID() = %v, want prefix 'INQ-'", id)
	}

	// Check if ID is long enough
	if len(id) < 20 {
		t.Errorf("GenerateInquiryID() = %v, length too short", id)
	}
}

func TestGenerateTransactionID(t *testing.T) {
	id := GenerateTransactionID()

	// Check if ID starts with "TXN-"
	if !strings.HasPrefix(id, "TXN-") {
		t.Errorf("GenerateTransactionID() = %v, want prefix 'TXN-'", id)
	}

	// Check if ID is long enough
	if len(id) < 20 {
		t.Errorf("GenerateTransactionID() = %v, length too short", id)
	}
}

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "Generate 6 character string",
			length: 6,
		},
		{
			name:   "Generate 10 character string",
			length: 10,
		},
		{
			name:   "Generate 1 character string",
			length: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateRandomString(tt.length)

			// Check length
			if len(got) != tt.length {
				t.Errorf("generateRandomString() = %v, want length %v", len(got), tt.length)
			}

			// Check if all characters are valid (A-Z, 0-9)
			for _, char := range got {
				if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
					t.Errorf("generateRandomString() = %v, contains invalid character %c", got, char)
				}
			}
		})
	}
}

func TestIDUniqueness(t *testing.T) {
	// Generate multiple IDs and check they are unique
	ids := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := GeneratePPSRequestID()
		if ids[id] {
			t.Errorf("GeneratePPSRequestID() generated duplicate ID: %v", id)
		}
		ids[id] = true
	}
}

func TestIDTimestampAccuracy(t *testing.T) {
	// Test that generated ID contains current timestamp
	now := time.Now()
	id := GeneratePPSRequestID()

	// Extract date and time parts
	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		t.Fatalf("Invalid ID format: %v", id)
	}

	datePart := parts[1] // YYYYMMDD
	timePart := parts[2] // HHMMSS

	// Parse date
	dateStr := datePart[:4] + "-" + datePart[4:6] + "-" + datePart[6:8]
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		t.Errorf("Failed to parse date from ID: %v", err)
	}

	// Parse time
	timeStr := timePart[:2] + ":" + timePart[2:4] + ":" + timePart[4:6]
	parsedTime, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		t.Errorf("Failed to parse time from ID: %v", err)
	}

	// Check if date matches (within same day)
	if parsedDate.Year() != now.Year() || parsedDate.Month() != now.Month() || parsedDate.Day() != now.Day() {
		t.Errorf("Date in ID %v does not match current date", id)
	}

	// Check if time is close (within 1 minute tolerance)
	timeDiff := parsedTime.Sub(time.Date(0, 1, 1, now.Hour(), now.Minute(), now.Second(), 0, time.UTC))
	if timeDiff < -time.Minute || timeDiff > time.Minute {
		t.Errorf("Time in ID %v does not match current time", id)
	}
}

