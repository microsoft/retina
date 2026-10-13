import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


images = load("prepare_images", "prepare-images.py")
report = load("test_report", "test-report.py")
LINUX = {("linux", "amd64"), ("linux", "arm64")}
WINDOWS = {("windows", "amd64")}


class CandidateTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.references = {
            key: f"example.azurecr.io/staging/{key.lower()}:v1.2.9-1@sha256:" + "a" * 64
            for key in images.ENV_NAMES
        }

    def platforms(self, reference):
        return WINDOWS if "windows" in reference else LINUX

    def metadata(self, key, version="1.2.9", reference=None):
        component = images.COMPONENTS.get(key, "retina-agent")
        directory = self.root / key
        directory.mkdir(exist_ok=True)
        (directory / "runtime-variables.json").write_text(json.dumps({
            "builtAnImage": True,
            "specDefinition": f"specs/containernetworking/{component}/{component}-{version}-1-specfile.yml",
            "stagingImage": reference or self.references[key],
        }))

    def collect(self):
        with patch.object(images, "login"), patch.object(images, "platforms", self.platforms):
            return images.collect(self.root, self.references["retinaAgentImage"], set())

    def test_complete_cohort(self):
        for key in self.references:
            self.metadata(key)
        actual, version = self.collect()
        self.assertEqual(actual, self.references)
        self.assertEqual(version, "1.2.9")

    def test_missing_companion(self):
        with patch.object(images, "login"), patch.object(images, "platforms", self.platforms):
            incomplete = dict(self.references)
            del incomplete["retinaInitImage"]
            with self.assertRaisesRegex(ValueError, "exactly these image keys"):
                images.validate(incomplete, "1.2.9", set())

    def test_mixed_versions(self):
        for key in self.references:
            self.metadata(key, "1.2.8" if key == "retinaInitImage" else "1.2.9")
        with self.assertRaisesRegex(ValueError, "exactly one Retina source version"):
            self.collect()

    def test_ambiguous_candidates(self):
        self.metadata("retinaAgentImage")
        extra = self.root / "extra"
        extra.mkdir()
        source = self.root / "retinaAgentImage" / "runtime-variables.json"
        (extra / source.name).write_text(source.read_text().replace("a" * 64, "b" * 64))
        with self.assertRaisesRegex(ValueError, "Multiple different candidates"):
            self.collect()

    def test_trigger_absent(self):
        self.metadata("retinaInitImage")
        with self.assertRaisesRegex(ValueError, "triggering candidate"):
            self.collect()

    def test_platform_requirement(self):
        with patch.object(images, "login"), patch.object(images, "platforms", return_value={("linux", "amd64")}):
            with self.assertRaisesRegex(ValueError, "missing required platforms"):
                images.validate(self.references, "1.2.9", set())

    def test_tag_is_pinned(self):
        with patch.object(images, "run", return_value="Name: example/image:v1\nDigest: sha256:" + "c" * 64):
            self.assertEqual(images.pin("example/image:v1"), "example/image:v1@sha256:" + "c" * 64)

    def test_invalid_reference(self):
        for reference in ("example/image:latest", "example/image@sha256:bad"):
            invalid = dict(self.references, retinaAgentImage=reference)
            with self.assertRaisesRegex(ValueError, "immutable"):
                images.validate(invalid, "1.2.9", set())

    def test_manifest_platforms(self):
        descriptors = [
            {"Descriptor": {"platform": {"os": os, "architecture": arch}}}
            for os, arch in LINUX
        ]
        with patch.object(images, "run", return_value=json.dumps(descriptors)):
            self.assertEqual(images.platforms("example/image"), LINUX)


class ReportTests(unittest.TestCase):
    def test_reports_pass_fail_skip_and_incomplete(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "events.jsonl"
            target = Path(directory) / "junit.xml"
            events = [
                {"Test": "passed", "Action": "pass", "Elapsed": 1},
                {"Test": "failed", "Action": "fail"},
                {"Test": "skipped", "Action": "skip"},
                {"Test": "incomplete", "Action": "run"},
            ]
            source.write_text("\n".join(json.dumps(event) for event in events))
            report.report(source, target)
            suite = report.ET.parse(target).getroot()
            self.assertEqual(suite.attrib["tests"], "4")
            self.assertEqual(suite.attrib["failures"], "2")
            self.assertEqual(suite.attrib["skipped"], "1")

    def test_empty_report_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "events.jsonl"
            source.write_text("")
            with self.assertRaisesRegex(ValueError, "No Go test events"):
                report.report(source, Path(directory) / "junit.xml")


if __name__ == "__main__":
    unittest.main()
