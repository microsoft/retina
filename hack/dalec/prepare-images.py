#!/usr/bin/env python3
"""Resolve a complete Dalec candidate set without rebuilding any images."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

COMPONENTS = {
    "retinaAgentImage": "retina-agent",
    "retinaInitImage": "retina-init",
    "retinaOperatorImage": "retina-operator",
    "kubectlRetinaImage": "kubectl-retina",
    "kubectlRetinaShellImage": "kubectl-retina-shell",
}
ENV_NAMES = {
    "retinaAgentImage": "RETINA_AGENT_IMAGE",
    "retinaWindowsAgentImage": "RETINA_WINDOWS_AGENT_IMAGE",
    "retinaInitImage": "RETINA_INIT_IMAGE",
    "retinaOperatorImage": "RETINA_OPERATOR_IMAGE",
    "kubectlRetinaImage": "KUBECTL_RETINA_IMAGE",
    "kubectlRetinaShellImage": "KUBECTL_RETINA_SHELL_IMAGE",
}
PINNED_IMAGE = re.compile(
    r"^[a-z0-9][a-z0-9.-]*(?::[0-9]+)?/[a-z0-9][a-z0-9._/-]*"
    r"(?::[A-Za-z0-9_][A-Za-z0-9_.-]*)?@sha256:[a-f0-9]{64}$"
)


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def login(image, registries):
    registry = image.split("/", 1)[0]
    if registry not in registries and registry.endswith(".azurecr.io"):
        run("az", "acr", "login", "--name", registry.removesuffix(".azurecr.io"))
        registries.add(registry)


def pin(image):
    if "@sha256:" not in image:
        output = run("docker", "buildx", "imagetools", "inspect", image)
        digest = re.search(r"^Digest:\s+(sha256:[a-f0-9]{64})\s*$", output, re.M)
        if not digest:
            raise ValueError(f"No immutable digest returned for {image}")
        image = image + "@" + digest.group(1)
    if not PINNED_IMAGE.fullmatch(image):
        raise ValueError(f"Invalid digest-pinned image reference: {image}")
    return image


def platforms(image):
    manifests = json.loads(run("docker", "manifest", "inspect", "--verbose", image))
    if isinstance(manifests, dict):
        manifests = [manifests]
    return {
        (item["Descriptor"]["platform"]["os"], item["Descriptor"]["platform"]["architecture"])
        for item in manifests
        if "platform" in item.get("Descriptor", {})
    }


def collect(metadata_dir, candidate, registries):
    images = {}
    versions = set()
    candidate_found = False
    for filename in sorted(Path(metadata_dir).rglob("runtime-variables.json")):
        metadata = json.loads(filename.read_text())
        if not metadata.get("builtAnImage"):
            continue
        spec = Path(metadata["specDefinition"])
        component = spec.parent.name
        if component not in COMPONENTS.values():
            continue
        match = re.fullmatch(re.escape(component) + r"-(.+)-(\d+)-specfile\.ya?ml", spec.name)
        if not match:
            raise ValueError(f"Cannot establish candidate version from {spec}")
        versions.add(match.group(1))
        original = metadata["stagingImage"]
        login(original, registries)
        image = pin(original)
        candidate_found |= candidate in (original, image)
        supported = platforms(image)
        key = next(key for key, name in COMPONENTS.items() if name == component)
        if component == "retina-agent":
            if ("linux", "amd64") not in supported:
                key = "retinaWindowsAgentImage"
        if key in images and images[key] != image:
            raise ValueError(f"Multiple different candidates for {key}")
        images[key] = image
    if len(versions) != 1:
        raise ValueError(f"Expected exactly one Retina source version, found {sorted(versions)}")
    if not candidate or not candidate_found:
        raise ValueError("The triggering candidate is not in this Dalec build's artifacts")
    return images, versions.pop()


def validate_input(images, version):
    if not isinstance(images, dict):
        raise ValueError("imageSet must be an object")
    if set(images) != set(ENV_NAMES):
        raise ValueError(f"Expected exactly these image keys: {sorted(ENV_NAMES)}")
    if not re.fullmatch(r"v?\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.-]+)?", version):
        raise ValueError("A valid release imageVersion is required")
    for key, image in images.items():
        if not isinstance(image, str) or not PINNED_IMAGE.fullmatch(image):
            raise ValueError(f"{key} must be an immutable sha256 image reference")


def validate(images, version, registries):
    validate_input(images, version)
    for key, image in images.items():
        login(image, registries)
        required = {("linux", "amd64"), ("linux", "arm64")}
        if key == "retinaWindowsAgentImage":
            required = {("windows", "amd64")}
        missing = required - platforms(image)
        if missing:
            raise ValueError(f"{key} is missing required platforms: {sorted(missing)}")


def main(validate_only=False):
    registries = set()
    images = json.loads(os.environ["IMAGE_SET_JSON"])
    version = os.environ.get("IMAGE_VERSION", "")
    build_id = os.environ.get("DALEC_BUILD_ID", "")
    if validate_only:
        if build_id:
            raise ValueError("Preflight requires a complete explicit imageSet, not build artifacts")
        validate_input(images, version)
        print("Candidate input is complete and digest-pinned")
        return
    if build_id:
        if images:
            raise ValueError("Do not mix manual image overrides with Dalec build artifacts")
        images, version = collect(
            os.environ["DALEC_METADATA_DIR"], os.environ["CANDIDATE_IMAGE"], registries
        )
    validate(images, version, registries)
    results = Path(os.environ["RESULTS_DIR"])
    results.mkdir(parents=True, exist_ok=True)
    manifest = {
        "images": images,
        "imageVersion": version,
        "dalecBuildId": build_id,
        "testCommit": os.environ["BUILD_SOURCEVERSION"],
    }
    (results / "candidate-images.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print(json.dumps(manifest, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--validate-only", action="store_true",
                        help="Check explicit candidate input without Azure or registry access")
    args = parser.parse_args()
    try:
        main(validate_only=args.validate_only)
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(f"Candidate validation failed: {error}", file=sys.stderr)
        sys.exit(1)
