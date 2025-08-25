// SPDX-FileCopyrightText: 2018 - 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"
)

func TestParseAutoDownloadTime(t *testing.T) {
	// now is 2025-08-20 12:00:00
	now := time.Date(2025, 8, 20, 12, 0, 0, 0, time.Local)

	tests := []struct {
		name           string
		hourMinute     string
		expectError    bool
		expectDateTime string
	}{
		// Valid cases
		{
			name:           "valid time 00:00, before now",
			hourMinute:     "00:00",
			expectError:    false,
			expectDateTime: "2025-08-20 00:00:00",
		},
		{
			name:           "valid time 12:30, after now",
			hourMinute:     "12:30",
			expectError:    false,
			expectDateTime: "2025-08-20 12:30:00",
		},
		{
			name:           "valid time 23:59, after now",
			hourMinute:     "23:59",
			expectError:    false,
			expectDateTime: "2025-08-20 23:59:00",
		},
		{
			name:           "valid time 09:05, before now",
			hourMinute:     "09:05",
			expectError:    false,
			expectDateTime: "2025-08-20 09:05:00",
		},
		{
			name:           "single digit hour",
			hourMinute:     "9:30",
			expectError:    false,
			expectDateTime: "2025-08-20 09:30:00",
		},
		// Invalid cases
		{
			name:        "invalid format without colon",
			hourMinute:  "1230",
			expectError: true,
		},
		{
			name:        "invalid format with seconds",
			hourMinute:  "12:30:45",
			expectError: true,
		},
		{
			name:        "invalid hour 24",
			hourMinute:  "24:00",
			expectError: true,
		},
		{
			name:        "invalid minute 60",
			hourMinute:  "12:60",
			expectError: true,
		},
		{
			name:        "invalid hour negative",
			hourMinute:  "-1:30",
			expectError: true,
		},
		{
			name:        "invalid minute negative",
			hourMinute:  "12:-5",
			expectError: true,
		},
		{
			name:        "empty string",
			hourMinute:  "",
			expectError: true,
		},
		{
			name:        "invalid format single digit minute",
			hourMinute:  "09:5",
			expectError: true,
		},
		{
			name:        "invalid characters",
			hourMinute:  "ab:cd",
			expectError: true,
		},
		{
			name:        "invalid format with space",
			hourMinute:  "12: 30",
			expectError: true,
		},
		{
			name:        "invalid format with extra characters",
			hourMinute:  "12:30am",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAutoDownloadTime(tt.hourMinute, now)
			t.Log("result: ", result)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none for input %q", tt.hourMinute)
				}
				// Check that zero time is returned on error
				if !result.IsZero() {
					t.Errorf("expected zero time on error but got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.hourMinute, err)
				}

				dt := result.Format(time.DateTime)
				if dt != tt.expectDateTime {
					t.Errorf("expected date time %s but got %s for input %q", tt.expectDateTime, dt, tt.hourMinute)
				}

				// Check that seconds are 0 (since we only parse hour:minute)
				if result.Second() != 0 {
					t.Errorf("expected second to be 0 but got %d for input %q", result.Second(), tt.hourMinute)
				}
			}
		})
	}
}

func TestParseAutoDownloadRange(t *testing.T) {
	// now is 2025-08-20 12:00:00
	now := time.Date(2025, 8, 20, 12, 0, 0, 0, time.Local)

	tests := []struct {
		name        string
		config      idleDownloadConfig
		expectError bool
		expectStart string
		expectEnd   string
	}{
		// Normal cases - begin time before end time
		{
			name: "normal range morning to afternoon",
			config: idleDownloadConfig{
				BeginTime: "09:00",
				EndTime:   "17:00",
			},
			expectError: false,
			expectStart: "2025-08-20 09:00:00",
			expectEnd:   "2025-08-20 17:00:00",
		},
		{
			name: "normal range early morning",
			config: idleDownloadConfig{
				BeginTime: "02:30",
				EndTime:   "06:45",
			},
			expectError: false,
			expectStart: "2025-08-20 02:30:00",
			expectEnd:   "2025-08-20 06:45:00",
		},
		{
			name: "same begin and end time",
			config: idleDownloadConfig{
				BeginTime: "12:00",
				EndTime:   "12:00",
			},
			expectError: false,
			expectStart: "2025-08-20 12:00:00",
			expectEnd:   "2025-08-20 12:00:00",
		},
		// Cross midnight cases - begin time after end time
		{
			name: "cross midnight evening to morning",
			config: idleDownloadConfig{
				BeginTime: "23:00",
				EndTime:   "03:00",
			},
			expectError: false,
			expectStart: "2025-08-20 23:00:00",
			expectEnd:   "2025-08-21 03:00:00", // next day
		},
		{
			name: "cross midnight late night to early morning",
			config: idleDownloadConfig{
				BeginTime: "22:30",
				EndTime:   "05:15",
			},
			expectError: false,
			expectStart: "2025-08-20 22:30:00",
			expectEnd:   "2025-08-21 05:15:00", // next day
		},
		{
			name: "cross midnight just after midnight",
			config: idleDownloadConfig{
				BeginTime: "23:59",
				EndTime:   "00:01",
			},
			expectError: false,
			expectStart: "2025-08-20 23:59:00",
			expectEnd:   "2025-08-21 00:01:00", // next day
		},
		// Error cases - invalid begin time
		{
			name: "invalid begin time format",
			config: idleDownloadConfig{
				BeginTime: "invalid",
				EndTime:   "17:00",
			},
			expectError: true,
		},
		{
			name: "invalid begin time hour",
			config: idleDownloadConfig{
				BeginTime: "25:00",
				EndTime:   "17:00",
			},
			expectError: true,
		},
		{
			name: "empty begin time",
			config: idleDownloadConfig{
				BeginTime: "",
				EndTime:   "17:00",
			},
			expectError: true,
		},
		// Error cases - invalid end time
		{
			name: "invalid end time format",
			config: idleDownloadConfig{
				BeginTime: "09:00",
				EndTime:   "invalid",
			},
			expectError: true,
		},
		{
			name: "invalid end time minute",
			config: idleDownloadConfig{
				BeginTime: "09:00",
				EndTime:   "17:60",
			},
			expectError: true,
		},
		{
			name: "empty end time",
			config: idleDownloadConfig{
				BeginTime: "09:00",
				EndTime:   "",
			},
			expectError: true,
		},
		// Error cases - both times invalid
		{
			name: "both times invalid",
			config: idleDownloadConfig{
				BeginTime: "invalid",
				EndTime:   "also_invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAutoDownloadRange(tt.config, now)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none for config %+v", tt.config)
				}
				// Check that zero TimeRange is returned on error
				if !result.Start.IsZero() || !result.End.IsZero() {
					t.Errorf("expected zero TimeRange on error but got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for config %+v: %v", tt.config, err)
				}

				startStr := result.Start.Format(time.DateTime)
				if startStr != tt.expectStart {
					t.Errorf("expected start time %s but got %s for config %+v", tt.expectStart, startStr, tt.config)
				}

				endStr := result.End.Format(time.DateTime)
				if endStr != tt.expectEnd {
					t.Errorf("expected end time %s but got %s for config %+v", tt.expectEnd, endStr, tt.config)
				}

				// Verify that the TimeRange was created correctly
				if result.Start.After(result.End) {
					t.Errorf("start time should not be after end time in result: start=%v, end=%v", result.Start, result.End)
				}
			}
		})
	}
}
