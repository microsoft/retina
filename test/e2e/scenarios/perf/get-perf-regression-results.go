package perf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
)

type TestInfo struct {
	Protocol string `json:"protocol"`
	Streams  int    `json:"streams"`
	Blksize  int    `json:"blksize"`
	Duration int    `json:"duration"`
}

type CPUUtilization struct {
	HostTotal   float64 `json:"host_total"`
	RemoteTotal float64 `json:"remote_total"`
}

type Result struct {
	TestInfo        TestInfo       `json:"test_info"`
	TotalThroughput float64        `json:"total_throughput"`
	MeanRTT         float64        `json:"mean_rtt"`
	MinRTT          float64        `json:"min_rtt"`
	MaxRTT          float64        `json:"max_rtt"`
	Retransmits     int            `json:"retransmits"`
	CPUUtilization  CPUUtilization `json:"cpu_utilization"`
	JitterMs        float64        `json:"jitter_ms"`
	LostPackets     int            `json:"lost_packets"`
	LostPercent     float64        `json:"lost_percent"`
	OutofOrder      int            `json:"out_of_order"`
}

type TestResult struct {
	Label  string `json:"label"`
	Result Result `json:"result"`
}

type RegressionResult struct {
	Label       string             `json:"label"`
	TestInfo    TestInfo           `json:"test_info"`
	Benchmark   map[string]float64 `json:"benchmark"`
	Result      map[string]float64 `json:"result"`
	Regressions map[string]float64 `json:"regressions"`
}

type GetNetworkRegressionResults struct {
	BaseResultsFile       string
	NewResultsFile        string
	RegressionResultsFile string
}

func (v *GetNetworkRegressionResults) Prevalidate() error {
	return nil
}

func (v *GetNetworkRegressionResults) Run() error {
	benchmarkResults, err := readJSONFile(v.BaseResultsFile)
	if err != nil {
		return errors.Wrap(err, "failed to read base results file")
	}

	newResults, err := readJSONFile(v.NewResultsFile)
	if err != nil {
		return errors.Wrap(err, "failed to read new results file")
	}

	if len(benchmarkResults) != len(newResults) {
		return errors.New("number of test results do not match")
	}

	regressionResults := make(map[string]*RegressionResult)

	for i := range benchmarkResults {
		benchmarkResult := benchmarkResults[i]
		newResult := newResults[i]

		if benchmarkResult.Label != newResult.Label {
			return errors.New("test labels do not match")
		}

		if _, exists := regressionResults[benchmarkResults[i].Label]; !exists {
			regressionResults[benchmarkResults[i].Label] = &RegressionResult{
				Label:       benchmarkResults[i].Label,
				TestInfo:    benchmarkResults[i].Result.TestInfo,
				Benchmark:   make(map[string]float64),
				Result:      make(map[string]float64),
				Regressions: make(map[string]float64),
			}
		}

		metrics := []struct {
			name      string
			benchmark float64
			result    float64
		}{
			{"total_throughput", benchmarkResult.Result.TotalThroughput, newResult.Result.TotalThroughput},
			{"mean_rtt_ms", benchmarkResult.Result.MeanRTT, newResult.Result.MeanRTT},
			{"min_rtt_ms", benchmarkResult.Result.MinRTT, newResult.Result.MinRTT},
			{"max_rtt_ms", benchmarkResult.Result.MaxRTT, newResult.Result.MaxRTT},
			{"retransmits", float64(benchmarkResult.Result.Retransmits), float64(newResult.Result.Retransmits)},
			{"jitter_ms", benchmarkResult.Result.JitterMs, newResult.Result.JitterMs},
			{"lost_packets", float64(benchmarkResult.Result.LostPackets), float64(newResult.Result.LostPackets)},
			{"lost_percent", benchmarkResult.Result.LostPercent, newResult.Result.LostPercent},
			{"out_of_order", float64(benchmarkResult.Result.OutofOrder), float64(newResult.Result.OutofOrder)},
			{"host_total_cpu", benchmarkResult.Result.CPUUtilization.HostTotal, newResult.Result.CPUUtilization.HostTotal},
			{"remote_total_cpu", benchmarkResult.Result.CPUUtilization.RemoteTotal, newResult.Result.CPUUtilization.RemoteTotal},
		}

		for _, metric := range metrics {
			if metric.benchmark != 0 && metric.result != 0 {
				regressionResults[benchmarkResult.Label].Benchmark[metric.name] = metric.benchmark
				regressionResults[benchmarkResult.Label].Result[metric.name] = metric.result
				regressionResults[benchmarkResult.Label].Regressions[metric.name] = calculateRegression(metric.benchmark, metric.result)
			}
		}
	}

	var results []RegressionResult
	for _, result := range regressionResults {
		results = append(results, *result)
	}

	file, err := os.Create(v.RegressionResultsFile)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create regression results file: %s", v.RegressionResultsFile))
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(results)
	if err != nil {
		return errors.Wrap(err, "failed to encode regression results")
	}

	return nil
}

func (v *GetNetworkRegressionResults) Stop() error {
	return nil
}

func readJSONFile(filename string) ([]TestResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var testCases []TestResult
	err = json.Unmarshal(byteValue, &testCases)
	if err != nil {
		return nil, err
	}

	return testCases, nil
}

func calculateRegression(old, new float64) float64 {
	if old == 0 {
		return 0
	}
	return ((new - old) / old) * 100
}
