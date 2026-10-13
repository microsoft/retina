#!/usr/bin/env python3
"""Convert Go JSON test events to an ADO-compatible JUnit report."""

import json
from pathlib import Path
import sys
import xml.etree.ElementTree as ET


def report(source, destination):
    tests = {}
    with open(source) as events:
        for line in events:
            event = json.loads(line)
            name = event.get("Test")
            if not name:
                continue
            test = tests.setdefault(name, {"output": [], "result": None, "elapsed": 0})
            if event["Action"] == "output":
                test["output"].append(event["Output"])
            elif event["Action"] in ("pass", "fail", "skip"):
                test["result"] = event["Action"]
                test["elapsed"] = event.get("Elapsed", 0)
    if not tests:
        raise ValueError("No Go test events were recorded")
    suite = ET.Element("testsuite", name="Retina Dalec E2E", tests=str(len(tests)))
    failures = skipped = 0
    for name, test in tests.items():
        case = ET.SubElement(suite, "testcase", name=name, time=str(test["elapsed"]))
        output = "".join(test["output"])
        if test["result"] not in ("pass", "skip"):
            failures += 1
            ET.SubElement(case, "failure", message="Test failed or did not finish").text = output
        elif test["result"] == "skip":
            skipped += 1
            ET.SubElement(case, "skipped")
        ET.SubElement(case, "system-out").text = output
    suite.set("failures", str(failures))
    suite.set("skipped", str(skipped))
    ET.ElementTree(suite).write(destination, encoding="utf-8", xml_declaration=True)


if __name__ == "__main__":
    report(Path(sys.argv[1]), Path(sys.argv[2]))
