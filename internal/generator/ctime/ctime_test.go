package ctime

import (
	"testing"
	"time"
)

func TestToNative(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		want    string
		wantErr bool
	}{
		{
			name:   "simple date",
			format: "%Y-%m-%d",
			want:   "2006-01-02",
		},
		{
			name:   "datetime iso8601",
			format: "%F %T",
			want:   "2006-01-02 15:04:05",
		},
		{
			name:   "ctime format",
			format: "%c",
			want:   "Mon Jan 02 15:04:05 2006",
		},
		{
			name:   "complex format",
			format: "%Y/%m/%d %H:%M:%S",
			want:   "2006/01/02 15:04:05",
		},
		{
			name:    "invalid directive",
			format:  "%Z %X",
			want:    "MST 15:04:05",
			wantErr: false,
		},
		{
			name:    "unsupported directive",
			format:  "%Q",
			wantErr: true,
		},
		{
			name:    "contains digits",
			format:  "%Y%m%d",
			want:    "20060102",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToNative(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToNative() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ToNative() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		input   time.Time
		want    string
		wantErr bool
	}{
		{
			name:   "ctime format",
			format: "%c",
			input:  time.Date(2026, time.January, 13, 15, 30, 45, 0, time.UTC),
			want:   "Tue Jan 13 15:30:45 2026",
		},
		{
			name:   "iso8601 format",
			format: "%F %T",
			input:  time.Date(2026, time.January, 13, 15, 30, 45, 0, time.UTC),
			want:   "2026-01-13 15:30:45",
		},
		{
			name:   "24-hour time",
			format: "%H:%M:%S",
			input:  time.Date(2026, time.January, 13, 14, 5, 3, 0, time.UTC),
			want:   "14:05:03",
		},
		{
			name:    "invalid directive",
			format:  "%Q",
			input:   time.Now(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format(tt.format, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Format() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		value   string
		want    time.Time
		wantErr bool
	}{
		{
			name:   "ctime format",
			format: "%c",
			value:  "Tue Jan 13 15:30:45 2026",
			want:   time.Date(2026, time.January, 13, 15, 30, 45, 0, time.UTC),
		},
		{
			name:   "iso8601 format",
			format: "%F %T",
			value:  "2026-01-13 15:30:45",
			want:   time.Date(2026, time.January, 13, 15, 30, 45, 0, time.UTC),
		},
		{
			name:    "invalid directive",
			format:  "%Q",
			value:   "anything",
			wantErr: true,
		},
		{
			name:    "mismatch value",
			format:  "%F",
			value:   "2026-13-45", // Invalid date
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.format, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
