import unittest

from app.time_utils import (
    TIMEZONE_BEIJING,
    TIMEZONE_UTC,
    convert_qso_datetime,
    normalize_qso_to_utc,
)


class TimeUtilsTests(unittest.TestCase):
    def test_beijing_to_utc_crosses_previous_day(self):
        self.assertEqual(
            convert_qso_datetime(
                "20260101",
                "0100",
                TIMEZONE_BEIJING,
                TIMEZONE_UTC,
            ),
            ("20251231", "1700"),
        )

    def test_utc_to_beijing_crosses_next_day(self):
        self.assertEqual(
            convert_qso_datetime(
                "20260101",
                "2000",
                TIMEZONE_UTC,
                TIMEZONE_BEIJING,
            ),
            ("20260102", "0400"),
        )

    def test_normalize_manual_entry_to_utc(self):
        normalized = normalize_qso_to_utc({
            "qso_date": "20260621",
            "time_on": "2230",
            "qso_type": "NORMAL",
            "input_timezone": TIMEZONE_BEIJING,
        })
        self.assertEqual(normalized["qso_date"], "20260621")
        self.assertEqual(normalized["time_on"], "1430")
        self.assertNotIn("input_timezone", normalized)


if __name__ == "__main__":
    unittest.main()
