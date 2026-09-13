package qso

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
)

func TestFreqToBand(t *testing.T) {
	cases := map[string]string{
		"14.270": "20m",
		"7.074":  "40m",
		"145.850": "2m",
		"":        "",
		"abc":     "",
		"0.5":     "",
		"9.000":   "",
	}
	for in, want := range cases {
		if got := FreqToBand(in); got != want {
			t.Errorf("FreqToBand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInsertAndListNormalizesTimezoneAndBand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qsl.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close(); runtime.GC() })
	if err := database.Init(db); err != nil {
		t.Fatalf("init: %v", err)
	}

	store := New(db)
	_, err = store.Insert(&Input{
		Call:          "bh7aa",
		QSODate:       "20260101",
		TimeOn:        "0100",
		InputTimezone: "Asia/Shanghai",
		QSOType:       "NORMAL",
		Freq:          "14.074",
		Mode:          "FT8",
		RstSent:       "59",
		RstRcvd:       "59",
		QSLStatus:     "未发送",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	res, err := store.List(Filters{}, 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 1 || len(res.Logs) != 1 {
		t.Fatalf("got total=%d logs=%d", res.Total, len(res.Logs))
	}
	log := res.Logs[0]
	if log.Call != "BH7AA" {
		t.Errorf("call = %q, want BH7AA", log.Call)
	}
	if got := strPtr(log.QSODate); got != "20251231" {
		t.Errorf("qso_date = %q, want 20251231 (UTC)", got)
	}
	if got := strPtr(log.TimeOn); got != "1700" {
		t.Errorf("time_on = %q, want 1700 (UTC)", got)
	}
	if got := strPtr(log.Band); got != "20m" {
		t.Errorf("band = %q, want 20m", got)
	}
}

func TestExportCSVHasBOMAndHeader(t *testing.T) {
	out, err := ExportCSV([]*QSO{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("\xef\xbb\xbf")) {
		t.Fatalf("CSV should start with UTF-8 BOM")
	}
	body := string(out[3:])
	if !bytes.Contains([]byte(body), []byte("CALL,DATE,TIME,BAND,FREQ,MODE")) {
		t.Fatalf("CSV missing header: %q", body)
	}
}
