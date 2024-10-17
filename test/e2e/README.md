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

### Reading Retina Perf Test Results

- All the performance related data are published as metrics to Azure App Insights. You can provide the Instrumentation Key for that as an env variable `AZURE_APP_INSIGHTS_KEY`.
- Metrics published:
  - **total_throughput**: The total amount of data successfully transferred over the network in a given time period.
  - **mean_rtt**: The average round-trip time (RTT) for packets sent over the network. [**Only TCP**]
  - **min_rtt**: The minimum round-trip time (RTT) observed for packets sent over the network. [**Only TCP**]
  - **max_rtt**: The maximum round-trip time (RTT) observed for packets sent over the network. [**Only TCP**]
  - **retransmits**: The number of packets that had to be retransmitted due to errors or loss. [**Only TCP**]
  - **jitter_ms**: The variation in packet arrival times, measured in milliseconds. [**Only UDP**]
  - **lost_packets**: The number of packets that were lost during transmission. [**Only UDP**]
  - **lost_percent**: The percentage of packets that were lost during transmission. [**Only UDP**]
  - **out_of_order**: The number of packets that arrived out of order. [**Only UDP**]
  - **host_total_cpu**: The total CPU utilization on the host machine.
  - **remote_total_cpu**: The total CPU utilization on the remote machine.
- All these metrics are published for each test case (We are running 4 as of now) with dimesntion name as `testCase`
- For each `testCase`, three sets of metrics are published under the dimension name `resultType` with values `benchmark`, `result`, and `regression`:
  - **benchmark**: Metrics collected from a baseline just after creating the cluster, used for comparison purposes. Retina is not installed at this point.
  - **result**: Metrics collected from the test run after installing Retina, representing the actual performance data.
  - **regression**: Metrics indicating any performance degradation compared to the benchmark. It is measured as percentage degradation from benchmark.
