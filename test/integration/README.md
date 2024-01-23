# Integration Tests

## Framework + Structure

We are using ginkgo to run and test retina for our integration tests. The MAKEFILE provide commands to run these tests and create test images. These integration tests will run on an AKS cluster which can be found here
`Retina/scripts/create_aks_cluster.sh`
When testing for advanced metrics, we should check if enablePodLevel is set. For an example, we can look at ./plugins/dropreson/dropreason_suite_test.go. This checks for the environment variable set before adding a new test entry for advanced metrics.

The azure pipeline was updated to accomdate these changes. The pipeline will create cluster for each profile, install retina with ENABLE_POD_LEVEL=true when the profile is "adv", and then it will run the integration tests.

/capture - integration tests related to capture command
/common - common tools we will use for our integration tests
/fixtures - these are static files which will be used for integration tests, such as crd configuraitons
/plugins - these are integration tests related to our ebpf programs

[This is the PR which has the integration test changes and pipeline changes](https://github.com/microsoft/retina/pull/)562/files

## Expectations

Each suite should:

* be idempotent
* not change cluster state (like delete retina-agent)
* no interference with other tests
