package main

import (
	"testing"
	"time"
)

func TestDeriveExpiresAt(t *testing.T) {
	const expIn = 3600
	now := time.Now().Unix()

	tests := []struct {
		name    string
		addedAt string
		want    int64
		wantAbs int64
		loose   bool
	}{
		{
			name:    "empty uses now",
			addedAt: "",
			loose:   true,
			wantAbs: now + expIn,
		},
		{
			name:    "unparseable falls back to now",
			addedAt: "not-a-time",
			loose:   true,
			wantAbs: now + expIn,
		},
		{
			name:    "RFC3339 utc",
			addedAt: "2026-05-12T22:08:41Z",
			want:    1778623721 + expIn,
		},
		{
			name:    "RFC3339 with tz",
			addedAt: "2026-05-12T22:08:41+07:00",
			want:    1778598521 + expIn,
		},
		{
			name:    "legacy local (no tz)",
			addedAt: "2026-05-12T22:08:41",
			loose:   true, // depends on test machine TZ; just assert reasonable delta
			wantAbs: mustParseLocal("2026-05-12T22:08:41").Unix() + expIn,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveExpiresAt(tc.addedAt, expIn)
			if tc.loose {
				// allow ±3s drift for time.Now() cases
				if abs(got-tc.wantAbs) > 3 {
					t.Errorf("deriveExpiresAt(%q, %d) = %d, want ≈ %d (delta %d)",
						tc.addedAt, int64(expIn), got, tc.wantAbs, got-tc.wantAbs)
				}
				return
			}
			if got != tc.want {
				t.Errorf("deriveExpiresAt(%q, %d) = %d, want %d",
					tc.addedAt, int64(expIn), got, tc.want)
			}
		})
	}
}

func mustParseLocal(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
