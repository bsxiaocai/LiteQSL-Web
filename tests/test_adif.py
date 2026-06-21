import unittest

from app.adif_parser import export_adif, parse_adif
from app.version import ADIF_VERSION, APP_VERSION


class AdifTests(unittest.TestCase):
    def test_standard_frequency_is_mhz(self):
        records = parse_adif(
            "<CALL:5>BH7AA <QSO_DATE:8>20260621 <TIME_ON:4>1200 "
            "<FREQ:6>14.074 <MODE:3>FT8 <QSL_SENT:1>Y <EOR>"
        )
        self.assertEqual(records[0]["freq"], "14.074")
        self.assertEqual(records[0]["qsl_status"], "已发送")

    def test_export_and_reimport_satellite_record(self):
        source = [{
            "call": "BH7AA",
            "qso_date": "20260621",
            "time_on": "1200",
            "band": "2m",
            "mode": "FM",
            "qso_type": "SAT",
            "tx_freq": "145.850",
            "rx_freq": "436.795",
            "sat_name": "SO-50",
            "sat_mode": "V/U",
            "qsl_status": "电子确认",
        }]

        exported = export_adif(source)
        self.assertIn(f"<ADIF_VER:{len(ADIF_VERSION)}>{ADIF_VERSION}", exported)
        self.assertIn(f"<PROGRAMVERSION:{len(APP_VERSION)}>{APP_VERSION}", exported)
        self.assertIn("<FREQ:6>145.85", exported)
        self.assertIn("<FREQ_RX:7>436.795", exported)
        self.assertIn("<SAT_MODE:3>V/U", exported)
        self.assertNotIn("<TX_FREQ:", exported)
        self.assertNotIn("<RX_FREQ:", exported)

        imported = parse_adif(exported)[0]
        self.assertEqual(imported["tx_freq"], "145.85")
        self.assertEqual(imported["rx_freq"], "436.795")
        self.assertEqual(imported["sat_mode"], "V/U")
        self.assertEqual(imported["qsl_status"], "电子确认")


if __name__ == "__main__":
    unittest.main()
