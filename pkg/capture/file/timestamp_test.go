package file

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNow(t *testing.T) {
	before := time.Now().UTC().Truncate(time.Second)
	result := Now()
	after := time.Now().UTC().Truncate(time.Second)

	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Time, before)
	assert.LessOrEqual(t, result.Time, after)
	assert.Equal(t, 0, result.Time.Nanosecond()) // ensure timestamp is truncated
}

func TestStringToTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  *metav1.Time
		wantError bool
	}{
		{
			name:      "valid timestamp",
			input:     "20250101120000UTC",
			expected:  &metav1.Time{Time: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)},
			wantError: false,
		},
		{
			name:      "another valid timestamp",
			input:     "20251231235959UTC",
			expected:  &metav1.Time{Time: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)},
			wantError: false,
		},
		{
			name:      "midnight timestamp",
			input:     "20250101000000UTC",
			expected:  &metav1.Time{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
			wantError: false,
		},
		{
			name:      "invalid format",
			input:     "2025-01-01 12:00:00",
			expected:  nil,
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			expected:  nil,
			wantError: true,
		},
		{
			name:      "invalid month",
			input:     "20251301120000UTC",
			expected:  nil,
			wantError: true,
		},
		{
			name:      "invalid day",
			input:     "20250132120000UTC",
			expected:  nil,
			wantError: true,
		},
		{
			name:      "invalid hour",
			input:     "20250101250000UTC",
			expected:  nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StringToTime(tt.input)
			if tt.wantError {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, tt.expected, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTimeToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *metav1.Time
		expected string
	}{
		{
			name:     "valid time",
			input:    &metav1.Time{Time: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)},
			expected: "20250101120000UTC",
		},
		{
			name:     "invalid month",
			input:    &metav1.Time{Time: time.Date(2025, 13, 1, 12, 0, 0, 0, time.UTC)},
			expected: "20260101120000UTC",
		},
		{
			name:     "invalid day",
			input:    &metav1.Time{Time: time.Date(2025, 1, 32, 12, 0, 0, 0, time.UTC)},
			expected: "20250201120000UTC",
		},
		{
			name:     "invalid hour",
			input:    &metav1.Time{Time: time.Date(2025, 1, 1, 25, 0, 0, 0, time.UTC)},
			expected: "20250102010000UTC",
		},
		{
			name:     "invalid minutes",
			input:    &metav1.Time{Time: time.Date(2025, 1, 1, 12, 61, 0, 0, time.UTC)},
			expected: "20250101130100UTC",
		},
		{
			name:     "invalid seconds",
			input:    &metav1.Time{Time: time.Date(2025, 1, 1, 12, 0, 61, 0, time.UTC)},
			expected: "20250101120101UTC",
		},
		{
			name:     "midnight",
			input:    &metav1.Time{Time: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
			expected: "20251201000000UTC",
		},
		{
			name:     "end of day",
			input:    &metav1.Time{Time: time.Date(2025, 1, 1, 23, 59, 59, 0, time.UTC)},
			expected: "20250101235959UTC",
		},
		{
			name:     "nil time pointer",
			input:    nil, // Should return a zero time string
			expected: "00010101000000UTC",
		},
		{
			name:     "zero time",
			input:    &metav1.Time{Time: time.Time{}}, // Same as nil case
			expected: "00010101000000UTC",
		},
		{
			name:     "time with different timezone",
			input:    &metav1.Time{Time: time.Date(2025, 6, 15, 14, 30, 45, 0, time.FixedZone("EST", -5*60*60))},
			expected: "20250615193045UTC", // Should be converted to UTC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TimeToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTimeToStringAndBack(t *testing.T) {
	originalTime := &metav1.Time{Time: time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)}

	timeString := TimeToString(originalTime)
	parsedTime, err := StringToTime(timeString)

	require.NoError(t, err)
	assert.True(t, originalTime.Time.Equal(parsedTime.Time))
}

func TestStringToTimeAndBack(t *testing.T) {
	originalString := "20250101123000UTC"

	parsedTime, err := StringToTime(originalString)
	require.NoError(t, err)

	timeString := TimeToString(parsedTime)
	assert.Equal(t, originalString, timeString)
}
