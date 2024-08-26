# Retina E2E

## Objectives

- Steps are reusable
- Steps parameters are saved to the context of the job
- Once written to the job context, the values are immutable
- Cluster resources used in code should be able to be generated to yaml for easy manual repro
- Avoid shell/ps calls wherever possible and use go libraries for typed parameters (avoid capturing error codes/stderr/stdout)

---

## Starter Example

When authoring tests, make sure to prefix the test name with `TestE2E` so that it is skipped by existing pipeline unit test framework.
For reference, see the `test-e2e` recipe in the root [Makefile](../../Makefile).

## Running E2E Test

You can execute all e2e tests in an AKS cluster. The image tag should be the image tag from the *Build Agent Images* pipeline on your PR.
You can also execute specific e2e tests (non-cilium and cilium) by setting `E2E_TEST_PATH`. If left empty, it will run all tests.
Example command:

```bash
export AZURE_SUBSCRIPTION_ID=<YOUR-SUBSCRIPTION> && \
export AZURE_LOCATION=<location> && \
IMAGE_TAG=<YOUR-IMAGE-TAG> make test-e2e
```

For sample test, please check out:
[the Retina E2E.](./scenarios/retina/drop/scenario.go)
