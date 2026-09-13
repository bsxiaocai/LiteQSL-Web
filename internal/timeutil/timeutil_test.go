package timeutil

import "testing"

// 以下用例与 v1.x tests/test_time_utils.py 保持一致。

func TestBeijingToUTCCrossesPreviousDay(t *testing.T) {
	d, tm, err := ConvertQsoDateTime("20260101", "0100", Beijing, UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != "20251231" || tm != "1700" {
		t.Fatalf("got (%q, %q), want (20251231, 1700)", d, tm)
	}
}

func TestUTCToBeijingCrossesNextDay(t *testing.T) {
	d, tm, err := ConvertQsoDateTime("20260101", "2000", UTC, Beijing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != "20260102" || tm != "0400" {
		t.Fatalf("got (%q, %q), want (20260102, 0400)", d, tm)
	}
}

func TestNormalizeManualEntryToUTC(t *testing.T) {
	normalized, err := NormalizeQsoToUTC(map[string]string{
		"qso_date":       "20260621",
		"time_on":        "2230",
		"qso_type":       "NORMAL",
		"input_timezone": Beijing,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized["qso_date"] != "20260621" || normalized["time_on"] != "1430" {
		t.Fatalf("got (%q, %q), want (20260621, 1430)", normalized["qso_date"], normalized["time_on"])
	}
	if _, ok := normalized["input_timezone"]; ok {
		t.Fatalf("input_timezone should be removed from result")
	}
}
