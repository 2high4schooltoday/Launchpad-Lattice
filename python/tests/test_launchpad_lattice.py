import pathlib
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

from launchpad_lattice import LatticeError, dumps, loads


class LaunchpadLatticeTests(unittest.TestCase):
    def test_parses_simple_payload_document(self) -> None:
        value = loads(
            """
lattice 1.

data:
  appId is "hello".
  retries is 3.
  enabled is true.
  architectures is list:
    item is "linux-x86_64".
    item is "linux-arm64".
  extension is record:
    bundle is uri "file:ext/demo.lext".
    sha256 is digest "sha256:abc".
"""
        )

        self.assertEqual(value["appId"], "hello")
        self.assertEqual(value["retries"], 3)
        self.assertEqual(value["enabled"], True)
        self.assertEqual(value["architectures"][0], "linux-x86_64")
        self.assertEqual(value["extension"]["bundle"], "file:ext/demo.lext")

    def test_ignores_non_data_sections_when_data_is_present(self) -> None:
        value = loads(
            """
lattice 1.

document is "org.example.demo".
schema is "org.example/demo/1".

data is Defaults:
  installDir is path "/opt/demo".
"""
        )

        self.assertEqual(value["installDir"], "/opt/demo")

    def test_renders_round_trip_payloads(self) -> None:
        original = {
            "appId": "hello",
            "enabled": True,
            "threshold": 2.5,
            "labels": {"io.launchpad.app/name": "hello"},
            "architectures": ["linux-x86_64", "linux-arm64"],
        }

        rendered = dumps(original)
        reparsed = loads(rendered)
        self.assertEqual(reparsed, original)

    def test_rejects_duplicate_keys(self) -> None:
        with self.assertRaises(LatticeError) as context:
            loads(
                """
lattice 1.

data:
  appId is "hello".
  appId is "duplicate".
"""
            )

        self.assertIn("duplicate key `appId`", str(context.exception))


if __name__ == "__main__":
    unittest.main()
