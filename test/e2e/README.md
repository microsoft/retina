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
For reference, see the `test-all` recipe in the root [Makefile](../../Makefile).

For sample test, please check out:
[the Retina E2E.](./scenarios/retina/drop/scenario.go)

## Sample VSCode `settings.json` for running with existing cluster

```json
"go.testFlags": [
    "-v",
    "-timeout=40m",
    "-tags=e2e",
    "-args",
    "-create-infra=false",
    "-delete-infra=false",
    "-image-namespace=retistrynamespace",
    "-image-registry=yourregistry",
    "-image-tag=yourtesttag",
],
```
