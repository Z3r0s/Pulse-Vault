import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("benchmark-cli.py")
SPEC = importlib.util.spec_from_file_location("benchmark_cli", SCRIPT)
benchmark_cli = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(benchmark_cli)


class RSSParsingTests(unittest.TestCase):
    def test_statm_uses_resident_pages(self):
        self.assertEqual(benchmark_cli.rss_bytes_from_statm("100 17 3 0 0 0 0", 4096), 69632)

    def test_statm_rejects_missing_resident_field(self):
        self.assertIsNone(benchmark_cli.rss_bytes_from_statm("100", 4096))

    def test_statm_rejects_invalid_resident_field(self):
        self.assertIsNone(benchmark_cli.rss_bytes_from_statm("100 resident", 4096))

    def test_linux_proc_fallback_reads_process_statm(self):
        with tempfile.TemporaryDirectory() as temporary:
            process_dir = Path(temporary) / "42"
            process_dir.mkdir()
            (process_dir / "statm").write_text("100 17 3 0 0 0 0", encoding="ascii")
            self.assertEqual(
                benchmark_cli.linux_process_rss_bytes(42, temporary, page_size=4096),
                69632,
            )


if __name__ == "__main__":
    unittest.main()
