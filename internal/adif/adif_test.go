package adif

import (
	"strings"
	"testing"

	"github.com/bsxiaocai/LiteQSL-Web/internal/qso"
)

// 以下用例对齐 v1.x tests/test_adif.py。

func TestStandardFrequencyIsMHz(t *testing.T) {
	records := Parse("<CALL:5>BH7AA <QSO_DATE:8>20260621 <TIME_ON:4>1200 " +
		"<FREQ:6>14.074 <MODE:3>FT8 <QSL_SENT:1>Y <EOR>")
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Freq != "14.074" {
		t.Fatalf("freq = %q, want 14.074", records[0].Freq)
	}
	if records[0].QSLStatus != "已发送" {
		t.Fatalf("qsl_status = %q, want 已发送", records[0].QSLStatus)
	}
}

func TestExportAndReimportSatelliteRecord(t *testing.T) {
	call := "BH7AA"
	qsoDate, timeOn := "20260621", "1200"
	band, mode := "2m", "FM"
	qsoType := "SAT"
	txFreq, rxFreq := "145.850", "436.795"
	satName, satMode := "SO-50", "V/U"
	qslStatus := "电子确认"

	source := []*qso.QSO{{
		Call:      call,
		QSODate:   &qsoDate,
		TimeOn:    &timeOn,
		Band:      &band,
		Mode:      &mode,
		QSOType:   &qsoType,
		TxFreq:    &txFreq,
		RxFreq:    &rxFreq,
		SatName:   &satName,
		SatMode:   &satMode,
		QSLStatus: &qslStatus,
	}}

	exported := Export(source)
	if !strings.Contains(exported, "<ADIF_VER:5>3.1.5") {
		t.Fatalf("missing ADIF_VER: %q", exported)
	}
	if !strings.Contains(exported, "<PROGRAMID:11>LiteQSL-Web") {
		t.Fatalf("missing PROGRAMID")
	}
	if !strings.Contains(exported, "<FREQ:6>145.85") {
		t.Fatalf("missing FREQ 145.85: %q", exported)
	}
	if !strings.Contains(exported, "<FREQ_RX:7>436.795") {
		t.Fatalf("missing FREQ_RX: %q", exported)
	}
	if !strings.Contains(exported, "<SAT_MODE:3>V/U") {
		t.Fatalf("missing SAT_MODE: %q", exported)
	}
	if strings.Contains(exported, "<TX_FREQ:") || strings.Contains(exported, "<RX_FREQ:") {
		t.Fatalf("should not emit TX_FREQ/RX_FREQ: %q", exported)
	}

	imported := Parse(exported)
	if len(imported) != 1 {
		t.Fatalf("reimport got %d records", len(imported))
	}
	if imported[0].TxFreq != "145.85" || imported[0].RxFreq != "436.795" {
		t.Fatalf("freq = (%q, %q)", imported[0].TxFreq, imported[0].RxFreq)
	}
	if imported[0].SatMode != "V/U" || imported[0].QSLStatus != "电子确认" {
		t.Fatalf("sat_mode=%q status=%q", imported[0].SatMode, imported[0].QSLStatus)
	}
}
