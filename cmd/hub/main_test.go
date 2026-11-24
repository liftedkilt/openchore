package main

import (
	"testing"
	"time"
)

func TestParseInviteTTL(t *testing.T) {
	tests := []struct {
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{raw: "", want: 0},
		{raw: "   ", want: 0},
		{raw: "12h", want: 12 * time.Hour},
		{raw: " 48h ", want: 48 * time.Hour},
		{raw: "30m", want: 30 * time.Minute},
		{raw: "0s", wantErr: true},
		{raw: "-1h", wantErr: true},
		{raw: "notaduration", wantErr: true},
	}
	for _, tc := range tests {
		got, err := parseInviteTTL(tc.raw)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseInviteTTL(%q) err=%v, wantErr=%v", tc.raw, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("parseInviteTTL(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
