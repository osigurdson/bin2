package server

import "testing"

func TestParseUploadRange(t *testing.T) {
	start, end, err := parseUploadRange("3-9")
	if err != nil {
		t.Fatalf("parseUploadRange: %v", err)
	}
	if start != 3 || end != 9 {
		t.Fatalf("range = %d-%d, want 3-9", start, end)
	}
}

func TestParseUploadRangeRejectsInvalid(t *testing.T) {
	tests := []string{
		"",
		"abc",
		"9-3",
		"-1-3",
	}

	for _, tc := range tests {
		if _, _, err := parseUploadRange(tc); err == nil {
			t.Fatalf("parseUploadRange(%q) unexpectedly succeeded", tc)
		}
	}
}

func TestUploadRangeMatchesContentLength(t *testing.T) {
	if !uploadRangeMatchesContentLength(3, 9, 7) {
		t.Fatalf("expected matching content length")
	}
	if uploadRangeMatchesContentLength(3, 9, 6) {
		t.Fatalf("expected mismatched content length to fail")
	}
	if !uploadRangeMatchesContentLength(3, 9, -1) {
		t.Fatalf("unknown content length should be accepted")
	}
}
