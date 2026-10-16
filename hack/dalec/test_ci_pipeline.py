"""Regression checks for the two modes of the existing OneBranch CI pipeline."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]


class PipelineTests(unittest.TestCase):
    def setUp(self):
        self.pipeline = yaml.safe_load((ROOT / ".azure-pipelines/ci.yml").read_text())
        self.parameters = {
            parameter["name"]: parameter for parameter in self.pipeline["parameters"]
        }
        self.stages = self.pipeline["extends"]["parameters"]["stages"]

    def test_dalec_mode_is_opt_in(self):
        self.assertIs(self.parameters["runDalecE2E"]["default"], False)
        self.assertIs(self.parameters["runE2EValidation"]["default"], False)

    def test_normal_stages_are_excluded_from_dalec_mode(self):
        normal = self.stages[0]["${{ if eq(parameters.runDalecE2E, false) }}"]
        self.assertEqual([stage["stage"] for stage in normal], ["build", "test"])
        self.assertEqual(normal[0]["displayName"], "Build")
        self.assertEqual(normal[1]["displayName"], "Run")

    def test_dalec_job_runs_on_host_and_without_build_dependencies(self):
        stage = self.stages[1]["${{ if eq(parameters.runDalecE2E, true) }}"][0]
        self.assertEqual(stage["dependsOn"], [])
        job = stage["jobs"][0]
        self.assertEqual(job["pool"], {"type": "linux", "runOnHost": True})
        steps = job["steps"]
        self.assertFalse(any(step.get("task") == "DownloadPipelineArtifact@2" for step in steps))
        validation = next(
            step for step in steps
            if step.get("displayName") == "Validate Dalec request before accessing Azure"
        )
        self.assertIn("--validate-only", validation["bash"])
        run = next(step for step in steps if step.get("displayName") == "Run Retina Dalec E2E")
        self.assertEqual(run["inputs"]["scriptPath"], "hack/dalec/run-e2e.sh")
        cleanup = next(
            step for step in steps
            if step.get("displayName") == "Clean up only this run's resource group"
        )
        self.assertEqual(
            cleanup["condition"], "and(always(), eq(variables['DalecE2EStarted'], 'true'))"
        )
        self.assertIn("retinaE2ERun", cleanup["inputs"]["inlineScript"])
        self.assertIn('[[ "$owner" == "$BUILD_BUILDID" ]]', cleanup["inputs"]["inlineScript"])
        results = next(step for step in steps if step.get("task") == "PublishTestResults@2")
        self.assertIs(results["inputs"]["failTaskOnMissingResultsFile"], True)
        self.assertFalse(any(step.get("task") == "PublishPipelineArtifact@1" for step in steps))
        self.assertEqual(job["variables"]["ob_artifactBaseName"], "retina-e2e")
        self.assertEqual(job["variables"]["ob_artifacts_main_condition"], "always()")
        self.assertTrue(job["variables"]["ob_outputDirectory"].endswith("/out"))

    def preflight(self, image_set, run_ci="False", reason="Manual"):
        stage = self.stages[1]["${{ if eq(parameters.runDalecE2E, true) }}"][0]
        step = next(
            step for step in stage["jobs"][0]["steps"]
            if step.get("displayName") == "Validate Dalec request before accessing Azure"
        )
        with tempfile.TemporaryDirectory() as directory:
            return subprocess.run(
                ["bash", "-c", step["bash"]], cwd=ROOT,
                env=dict(os.environ, IMAGE_SET_JSON=json.dumps(image_set),
                         IMAGE_VERSION="1.2.9", DALEC_BUILD_ID="",
                         RUN_CI_E2E=run_ci, BUILD_REASON=reason, RESULTS_DIR=directory),
                text=True, capture_output=True, check=False,
            )

    def test_empty_candidate_set_fails_preflight(self):
        result = self.preflight({})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("exactly these image keys", result.stderr)

    def test_conflicting_modes_fail_preflight(self):
        result = self.preflight({}, run_ci="True")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not both", result.stderr)

    def test_automatic_triggers_cannot_run_dalec_mode(self):
        result = self.preflight({}, reason="PullRequest")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("must be queued explicitly", result.stderr)

    def test_complete_pinned_input_passes_preflight_without_network_access(self):
        image_set = {
            key: "registry.azurecr.io/staging/example@sha256:" + "a" * 64
            for key in (
                "retinaAgentImage", "retinaWindowsAgentImage", "retinaInitImage",
                "retinaOperatorImage", "kubectlRetinaImage", "kubectlRetinaShellImage",
            )
        }
        result = self.preflight(image_set)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
