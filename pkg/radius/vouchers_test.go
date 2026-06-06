package radius

import (
	"regexp"
	"testing"
)

func TestGenerateRandomCode(t *testing.T) {
	tests := []struct {
		name       string
		length     int
		codeType   string
		pattern    string
	}{
		{
			name:     "Alphanumeric code",
			length:   10,
			codeType: "alphanumeric",
			pattern:  "^[ABCDEFGHJKLMNPQRSTUVWXYZ2-9]{10}$",
		},
		{
			name:     "Numeric code",
			length:   6,
			codeType: "numbers",
			pattern:  "^[0-9]{6}$",
		},
		{
			name:     "Alphabetic code",
			length:   8,
			codeType: "letters",
			pattern:  "^[A-Z]{8}$",
		},
		{
			name:     "Default fallback type",
			length:   5,
			codeType: "invalid-type",
			pattern:  "^[ABCDEFGHJKLMNPQRSTUVWXYZ2-9]{5}$",
		},
		{
			name:     "Default fallback length",
			length:   0,
			codeType: "numbers",
			pattern:  "^[0-9]{10}$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := generateRandomCode(tt.length, tt.codeType)
			
			// Validate length
			expectedLength := tt.length
			if tt.length <= 0 {
				expectedLength = 10
			}
			if len(code) != expectedLength {
				t.Errorf("expected length %d, got %d for code: %s", expectedLength, len(code), code)
			}

			// Validate characters match pattern
			matched, err := regexp.MatchString(tt.pattern, code)
			if err != nil {
				t.Fatalf("regex failed: %v", err)
			}
			if !matched {
				t.Errorf("code %s did not match pattern %s", code, tt.pattern)
			}
		})
	}
}
