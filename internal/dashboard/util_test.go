package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestHumanInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "small positive",
			input:    123,
			expected: "123",
		},
		{
			name:     "small negative",
			input:    -456,
			expected: "-456",
		},
		{
			name:     "thousands",
			input:    1234,
			expected: "1,234",
		},
		{
			name:     "millions",
			input:    1234567,
			expected: "1,234,567",
		},
		{
			name:     "billions",
			input:    1234567890,
			expected: "1,234,567,890",
		},
		{
			name:     "negative millions",
			input:    -1234567,
			expected: "-1,234,567",
		},
		{
			name:     "exactly 1000",
			input:    1000,
			expected: "1,000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanInt(tt.input)
			if result != tt.expected {
				t.Errorf("HumanInt(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatLargeNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "below 1000",
			input:    999,
			expected: "999",
		},
		{
			name:     "exactly 1000 - threshold K",
			input:    1000,
			expected: "1.00K",
		},
		{
			name:     "1500 - K",
			input:    1500,
			expected: "1.50K",
		},
		{
			name:     "millions - M",
			input:    2500000,
			expected: "2.50M",
		},
		{
			name:     "billions - B",
			input:    3500000000,
			expected: "3.50B",
		},
		{
			name:     "trillions - T",
			input:    4500000000000,
			expected: "4.50T",
		},
		{
			name:     "negative K",
			input:    -1500,
			expected: "-1.50K",
		},
		{
			name:     "negative M",
			input:    -2500000,
			expected: "-2.50M",
		},
		{
			name:     "negative B",
			input:    -3500000000,
			expected: "-3.50B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatLargeNumber(tt.input)
			if result != tt.expected {
				t.Errorf("FormatLargeNumber(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "em dash placeholder",
			input:    "—",
			expected: "—",
		},
		{
			name:     "hyphen placeholder",
			input:    "-",
			expected: "-",
		},
		{
			name:     "small integer no commas",
			input:    "123",
			expected: "123",
		},
		{
			name:     "small decimal no commas",
			input:    "123.45",
			expected: "123.45",
		},
		{
			name:     "thousands with decimal",
			input:    "1234.56",
			expected: "1,234.56",
		},
		{
			name:     "millions with decimal",
			input:    "902030185089.93",
			expected: "902,030,185,089.93",
		},
		{
			name:     "integer only thousands",
			input:    "123456",
			expected: "123,456",
		},
		{
			name:     "exactly 1000",
			input:    "1000",
			expected: "1,000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFloat(tt.input)
			if result != tt.expected {
				t.Errorf("FormatFloat(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatSmartNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "em dash placeholder",
			input:    "—",
			expected: "—",
		},
		{
			name:     "hyphen placeholder",
			input:    "-",
			expected: "-",
		},
		{
			name:     "invalid number returns original",
			input:    "not-a-number",
			expected: "not-a-number",
		},
		{
			name:     "small number with commas",
			input:    "123456.78",
			expected: "123,456.78",
		},
		{
			name:     "billion abbreviated",
			input:    "1500000000.5",
			expected: "1.50B",
		},
		{
			name:     "trillion abbreviated",
			input:    "2500000000000",
			expected: "2.50T",
		},
		{
			name:     "negative billion",
			input:    "-1500000000",
			expected: "-1.50B",
		},
		{
			name:     "below billion uses commas",
			input:    "999999999",
			expected: "999,999,999",
		},
		{
			name:     "exactly 1 billion",
			input:    "1000000000",
			expected: "1.00B",
		},
		{
			name:     "number with existing commas - reformats correctly",
			input:    "1234567.00",
			expected: "1,234,567.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSmartNumber(tt.input)
			if result != tt.expected {
				t.Errorf("FormatSmartNumber(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "zero",
			input:    0.0,
			expected: "0%",
		},
		{
			name:     "half",
			input:    0.5,
			expected: "50%",
		},
		{
			name:     "one (100%)",
			input:    1.0,
			expected: "100%",
		},
		{
			name:     "negative clamped to 0%",
			input:    -0.5,
			expected: "0.0%",
		},
		{
			name:     "greater than 1 clamped to 100%",
			input:    1.5,
			expected: "100.0%",
		},
		{
			name:     "small fraction",
			input:    0.00123,
			expected: "0.123%",
		},
		{
			name:     "decimal percentage",
			input:    0.123,
			expected: "12.3%",
		},
		{
			name:     "very small fraction",
			input:    0.000001,
			expected: "0.0001%",
		},
		{
			name:     "trailing zeros removed",
			input:    0.25,
			expected: "25%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Percent(tt.input)
			if result != tt.expected {
				t.Errorf("Percent(%f) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		fraction float64
		width    int
		noEmoji  bool
		validate func(t *testing.T, result string)
	}{
		{
			name:     "zero progress - ASCII",
			fraction: 0.0,
			width:    10,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				expected := "[        ]"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "half progress - ASCII",
			fraction: 0.5,
			width:    10,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				expected := "[====    ]"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "full progress - ASCII",
			fraction: 1.0,
			width:    10,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				expected := "[========]"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "zero progress - Unicode",
			fraction: 0.0,
			width:    10,
			noEmoji:  false,
			validate: func(t *testing.T, result string) {
				expected := "░░░░░░░░░░"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "half progress - Unicode",
			fraction: 0.5,
			width:    10,
			noEmoji:  false,
			validate: func(t *testing.T, result string) {
				expected := "█████░░░░░"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "full progress - Unicode",
			fraction: 1.0,
			width:    10,
			noEmoji:  false,
			validate: func(t *testing.T, result string) {
				expected := "██████████"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "negative fraction clamped to 0",
			fraction: -0.5,
			width:    10,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				expected := "[        ]"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "fraction > 1 clamped to 1",
			fraction: 1.5,
			width:    10,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				expected := "[========]"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "too narrow width - returns percentage",
			fraction: 0.75,
			width:    2,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				expected := "75%"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "width 1 - returns percentage",
			fraction: 0.33,
			width:    1,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				expected := "33%"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "width 0 - returns percentage",
			fraction: 0.50,
			width:    0,
			noEmoji:  false,
			validate: func(t *testing.T, result string) {
				expected := "50%"
				if result != expected {
					t.Errorf("got %q, want %q", result, expected)
				}
			},
		},
		{
			name:     "various width - ASCII",
			fraction: 0.3,
			width:    20,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				if !strings.HasPrefix(result, "[") || !strings.HasSuffix(result, "]") {
					t.Errorf("expected brackets, got %q", result)
				}
				if len(result) != 20 {
					t.Errorf("expected length 20, got %d", len(result))
				}
			},
		},
		{
			name:     "various width - Unicode",
			fraction: 0.7,
			width:    15,
			noEmoji:  false,
			validate: func(t *testing.T, result string) {
				runes := []rune(result)
				if len(runes) != 15 {
					t.Errorf("expected 15 runes, got %d", len(runes))
				}
			},
		},
		{
			name:     "edge case - width exactly 3 ASCII",
			fraction: 0.5,
			width:    3,
			noEmoji:  true,
			validate: func(t *testing.T, result string) {
				// Width 3 with ASCII: brackets take 2, leaving 1 for bar
				if result != "[ ]" && result != "[=]" {
					t.Errorf("expected '[ ]' or '[=]', got %q", result)
				}
			},
		},
		{
			name:     "edge case - width exactly 3 Unicode",
			fraction: 0.5,
			width:    3,
			noEmoji:  false,
			validate: func(t *testing.T, result string) {
				runes := []rune(result)
				if len(runes) != 3 {
					t.Errorf("expected 3 runes, got %d", len(runes))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProgressBar(tt.fraction, tt.width, tt.noEmoji)
			tt.validate(t, result)
		})
	}
}

func TestDurationShort(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "0s",
		},
		{
			name:     "30 seconds",
			input:    30 * time.Second,
			expected: "30s",
		},
		{
			name:     "59 seconds",
			input:    59 * time.Second,
			expected: "59s",
		},
		{
			name:     "exactly 1 minute",
			input:    time.Minute,
			expected: "1m",
		},
		{
			name:     "5 minutes",
			input:    5 * time.Minute,
			expected: "5m",
		},
		{
			name:     "59 minutes",
			input:    59 * time.Minute,
			expected: "59m",
		},
		{
			name:     "exactly 1 hour",
			input:    time.Hour,
			expected: "1h",
		},
		{
			name:     "1 hour 30 minutes",
			input:    time.Hour + 30*time.Minute,
			expected: "1h30m",
		},
		{
			name:     "5 hours",
			input:    5 * time.Hour,
			expected: "5h",
		},
		{
			name:     "23 hours 45 minutes",
			input:    23*time.Hour + 45*time.Minute,
			expected: "23h45m",
		},
		{
			name:     "exactly 1 day",
			input:    24 * time.Hour,
			expected: "1d",
		},
		{
			name:     "2 days 5 hours",
			input:    2*24*time.Hour + 5*time.Hour,
			expected: "2d5h",
		},
		{
			name:     "7 days",
			input:    7 * 24 * time.Hour,
			expected: "7d",
		},
		{
			name:     "30 days 12 hours",
			input:    30*24*time.Hour + 12*time.Hour,
			expected: "30d12h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DurationShort(tt.input)
			if result != tt.expected {
				t.Errorf("DurationShort(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, result string)
	}{
		{
			name:  "empty string",
			input: "",
			validate: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			},
		},
		{
			name:  "invalid timestamp",
			input: "not-a-timestamp",
			validate: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string for invalid input, got %q", result)
				}
			},
		},
		{
			name:  "valid RFC3339 timestamp",
			input: "2024-01-15T14:30:00Z",
			validate: func(t *testing.T, result string) {
				if result == "" {
					t.Error("expected non-empty result for valid timestamp")
				}
				// Should contain month abbreviation
				if !strings.Contains(result, "Jan") {
					t.Errorf("expected month abbreviation, got %q", result)
				}
				// Should contain day
				if !strings.Contains(result, "15") {
					t.Errorf("expected day 15, got %q", result)
				}
			},
		},
		{
			name:  "valid RFC3339Nano timestamp",
			input: "2024-06-20T09:15:30.123456789Z",
			validate: func(t *testing.T, result string) {
				if result == "" {
					t.Error("expected non-empty result for valid timestamp")
				}
				// Should contain month abbreviation
				if !strings.Contains(result, "Jun") {
					t.Errorf("expected month abbreviation, got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimestamp(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestTimeUntil(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, result string)
	}{
		{
			name:  "empty string",
			input: "",
			validate: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			},
		},
		{
			name:  "invalid timestamp",
			input: "invalid",
			validate: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string for invalid input, got %q", result)
				}
			},
		},
		{
			name:  "past timestamp",
			input: "2020-01-01T00:00:00Z",
			validate: func(t *testing.T, result string) {
				if result != "0s" {
					t.Errorf("expected '0s' for past timestamp, got %q", result)
				}
			},
		},
		{
			name:  "future timestamp",
			input: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
			validate: func(t *testing.T, result string) {
				if result == "" || result == "0s" {
					t.Errorf("expected duration for future timestamp, got %q", result)
				}
				// Should contain 'm' for minutes
				if !strings.Contains(result, "m") && !strings.Contains(result, "s") {
					t.Errorf("expected duration format, got %q", result)
				}
			},
		},
		{
			name:  "far future timestamp",
			input: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			validate: func(t *testing.T, result string) {
				if result == "" {
					t.Error("expected non-empty result for future timestamp")
				}
				// Should contain 'd' for days or 'h' for hours
				if !strings.Contains(result, "d") && !strings.Contains(result, "h") {
					t.Errorf("expected duration format with days or hours, got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TimeUntil(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestETACalculator(t *testing.T) {
	t.Run("NewETACalculator", func(t *testing.T) {
		calc := NewETACalculator()
		if calc == nil {
			t.Fatal("expected non-nil calculator")
		}
		if calc.maxSamples != 20 {
			t.Errorf("expected maxSamples=20, got %d", calc.maxSamples)
		}
	})

	t.Run("Calculate with insufficient data", func(t *testing.T) {
		calc := NewETACalculator()
		result := calc.Calculate()
		if result != "calculating..." {
			t.Errorf("expected 'calculating...' with no samples, got %q", result)
		}

		calc.AddSample(1000)
		result = calc.Calculate()
		if result != "calculating..." {
			t.Errorf("expected 'calculating...' with 1 sample, got %q", result)
		}
	})

	t.Run("Calculate with no progress (stalled)", func(t *testing.T) {
		calc := NewETACalculator()

		// Add multiple samples with no progress
		calc.AddSample(1000)
		time.Sleep(10 * time.Millisecond)
		calc.AddSample(1000)
		time.Sleep(10 * time.Millisecond)
		calc.AddSample(1000)

		result := calc.Calculate()
		if result != "calculating..." {
			t.Errorf("expected 'calculating...' for no progress, got %q", result)
		}
	})

	t.Run("Calculate with progress", func(t *testing.T) {
		calc := NewETACalculator()

		// Simulate syncing: blocks behind decreasing over time
		calc.AddSample(1000)
		time.Sleep(100 * time.Millisecond)
		calc.AddSample(900)
		time.Sleep(100 * time.Millisecond)
		calc.AddSample(800)

		result := calc.Calculate()
		// Should return a duration string (not "calculating..." or "stalled")
		if result == "" || result == "calculating..." {
			t.Errorf("expected duration with progress, got %q", result)
		}
	})

	t.Run("Calculate when synced (0 blocks behind)", func(t *testing.T) {
		calc := NewETACalculator()

		calc.AddSample(100)
		time.Sleep(100 * time.Millisecond)
		calc.AddSample(0)

		result := calc.Calculate()
		if result != "0s" {
			t.Errorf("expected '0s' when synced, got %q", result)
		}
	})

	t.Run("AddSample limits samples to maxSamples", func(t *testing.T) {
		calc := NewETACalculator()

		// Add more than maxSamples
		for i := 0; i < 25; i++ {
			calc.AddSample(int64(1000 - i*10))
			time.Sleep(time.Millisecond)
		}

		if len(calc.samples) > calc.maxSamples {
			t.Errorf("expected at most %d samples, got %d", calc.maxSamples, len(calc.samples))
		}
	})

	t.Run("Calculate with very fast sync", func(t *testing.T) {
		calc := NewETACalculator()

		calc.AddSample(100)
		time.Sleep(100 * time.Millisecond)
		calc.AddSample(10)

		result := calc.Calculate()
		// Should return a short duration
		if result == "" || result == "calculating..." {
			t.Errorf("expected duration, got %q", result)
		}
	})

	t.Run("Calculate caps at >1y for very long ETA", func(t *testing.T) {
		calc := NewETACalculator()

		// Simulate extremely slow progress
		calc.AddSample(1000000000) // 1 billion blocks behind
		time.Sleep(100 * time.Millisecond)
		calc.AddSample(999999999) // Only 1 block of progress

		result := calc.Calculate()
		if result != ">1y" {
			t.Errorf("expected '>1y' for extremely long ETA, got %q", result)
		}
	})

	t.Run("Calculate detects stalled sync after 30s", func(t *testing.T) {
		calc := NewETACalculator()

		// Add initial sample with progress to set lastProgress
		calc.AddSample(1000)
		time.Sleep(10 * time.Millisecond)
		calc.AddSample(900)

		// Set lastProgress to more than 30s ago
		calc.lastProgress = time.Now().Add(-35 * time.Second)

		// Now add samples that show no net progress from first to last
		// Replace samples to show stalled state
		time.Sleep(100 * time.Millisecond)
		calc.samples = []struct {
			blocksBehind int64
			timestamp    time.Time
		}{
			{900, time.Now().Add(-200 * time.Millisecond)},
			{900, time.Now()},
		}

		result := calc.Calculate()
		if result != "stalled" {
			t.Errorf("expected 'stalled' after 30s of no progress, got %q", result)
		}
	})

	t.Run("Calculate with timeDelta too small", func(t *testing.T) {
		calc := NewETACalculator()

		// Add samples with almost no time difference
		now := time.Now()
		calc.samples = append(calc.samples, struct {
			blocksBehind int64
			timestamp    time.Time
		}{1000, now})
		calc.samples = append(calc.samples, struct {
			blocksBehind int64
			timestamp    time.Time
		}{900, now.Add(50 * time.Millisecond)})

		result := calc.Calculate()
		if result != "calculating..." {
			t.Errorf("expected 'calculating...' with small timeDelta, got %q", result)
		}
	})

	t.Run("Calculate with negative rate", func(t *testing.T) {
		calc := NewETACalculator()

		// Add samples where blocks behind is increasing (negative progress)
		calc.AddSample(100)
		time.Sleep(100 * time.Millisecond)
		calc.AddSample(200) // Blocks behind increased

		result := calc.Calculate()
		// Should return calculating... because rate is negative
		if result != "calculating..." {
			t.Errorf("expected 'calculating...' with negative rate, got %q", result)
		}
	})
}

func TestNewIcons(t *testing.T) {
	t.Run("ASCII mode (noEmoji=true)", func(t *testing.T) {
		icons := NewIcons(true)

		if icons.OK != "[OK]" {
			t.Errorf("expected OK='[OK]', got %q", icons.OK)
		}
		if icons.Warn != "[!]" {
			t.Errorf("expected Warn='[!]', got %q", icons.Warn)
		}
		if icons.Err != "[X]" {
			t.Errorf("expected Err='[X]', got %q", icons.Err)
		}
		if icons.Peer != "#" {
			t.Errorf("expected Peer='#', got %q", icons.Peer)
		}
		if icons.Block != "#" {
			t.Errorf("expected Block='#', got %q", icons.Block)
		}
		if icons.Unknown != "[?]" {
			t.Errorf("expected Unknown='[?]', got %q", icons.Unknown)
		}
	})

	t.Run("emoji mode (noEmoji=false)", func(t *testing.T) {
		icons := NewIcons(false)

		if icons.OK != "✓" {
			t.Errorf("expected OK='✓', got %q", icons.OK)
		}
		if icons.Warn != "⚠" {
			t.Errorf("expected Warn='⚠', got %q", icons.Warn)
		}
		if icons.Err != "✗" {
			t.Errorf("expected Err='✗', got %q", icons.Err)
		}
		if icons.Peer != "🔗" {
			t.Errorf("expected Peer='🔗', got %q", icons.Peer)
		}
		if icons.Block != "📦" {
			t.Errorf("expected Block='📦', got %q", icons.Block)
		}
		if icons.Unknown != "◯" {
			t.Errorf("expected Unknown='◯', got %q", icons.Unknown)
		}
	})
}

func TestFormatTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		width    int
		validate func(t *testing.T, result string)
	}{
		{
			name:  "simple title",
			title: "test",
			width: 20,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "TEST") {
					t.Errorf("expected uppercase, got %q", result)
				}
			},
		},
		{
			name:  "title is uppercased",
			title: "hello world",
			width: 30,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "HELLO WORLD") {
					t.Errorf("expected 'HELLO WORLD', got %q", result)
				}
			},
		},
		{
			name:  "various widths",
			title: "status",
			width: 10,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "STATUS") {
					t.Errorf("expected 'STATUS', got %q", result)
				}
			},
		},
		{
			name:  "wide width",
			title: "info",
			width: 50,
			validate: func(t *testing.T, result string) {
				if !strings.Contains(result, "INFO") {
					t.Errorf("expected 'INFO', got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTitle(tt.title, tt.width)
			tt.validate(t, result)
		})
	}
}
