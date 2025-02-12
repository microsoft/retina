package perf

import (
	"encoding/json"
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

type DeltaResult struct {
	Label    string             `json:"label"`
	TestInfo TestInfo           `json:"test_info"`
	Baseline map[string]float64 `json:"baseline"`
	Result   map[string]float64 `json:"result"`
	Deltas   map[string]float64 `json:"regressions"`
}

type GetNetworkDeltaResults struct {
	BaseResultsFile  string
	NewResultsFile   string
	DeltaResultsFile string
}

func (v *GetNetworkDeltaResults) Prevalidate() error {
	return nil
}

func (v *GetNetworkDeltaResults) Run() error {
	baselineResults, err := readJSONFile(v.BaseResultsFile)
	if err != nil {
		return errors.Wrap(err, "failed to read base results file")
	}

	newResults, err := readJSONFile(v.NewResultsFile)
	if err != nil {
		return errors.Wrap(err, "failed to read new results file")
	}

	if len(baselineResults) != len(newResults) {
		return errors.New("number of test results do not match")
	}

	regressionResults := make(map[string]*DeltaResult)

	for i := range baselineResults {
		baselineResult := baselineResults[i]
		newResult := newResults[i]

		if baselineResult.Label != newResult.Label {
			return errors.New("test labels do not match")
		}

		if _, exists := regressionResults[baselineResults[i].Label]; !exists {
			regressionResults[baselineResults[i].Label] = &DeltaResult{
				Label:    baselineResults[i].Label,
				TestInfo: baselineResults[i].Result.TestInfo,
				Baseline: make(map[string]float64),
				Result:   make(map[string]float64),
				Deltas:   make(map[string]float64),
			}
		}

		metrics := []struct {
			name     string
			baseline float64
			result   float64
		}{
			{"total_throughput_gbits_sec", baselineResult.Result.TotalThroughput, newResult.Result.TotalThroughput},
			{"mean_rtt_ms", baselineResult.Result.MeanRTT, newResult.Result.MeanRTT},
			{"min_rtt_ms", baselineResult.Result.MinRTT, newResult.Result.MinRTT},
			{"max_rtt_ms", baselineResult.Result.MaxRTT, newResult.Result.MaxRTT},
			{"retransmits", float64(baselineResult.Result.Retransmits), float64(newResult.Result.Retransmits)},
			{"jitter_ms", baselineResult.Result.JitterMs, newResult.Result.JitterMs},
			{"lost_packets", float64(baselineResult.Result.LostPackets), float64(newResult.Result.LostPackets)},
			{"lost_percent", baselineResult.Result.LostPercent, newResult.Result.LostPercent},
			{"out_of_order", float64(baselineResult.Result.OutofOrder), float64(newResult.Result.OutofOrder)},
			{"host_total_cpu", baselineResult.Result.CPUUtilization.HostTotal, newResult.Result.CPUUtilization.HostTotal},
			{"remote_total_cpu", baselineResult.Result.CPUUtilization.RemoteTotal, newResult.Result.CPUUtilization.RemoteTotal},
		}

		for _, metric := range metrics {
			if metric.baseline != 0 || metric.result != 0 {
				regressionResults[baselineResult.Label].Baseline[metric.name] = metric.baseline
				regressionResults[baselineResult.Label].Result[metric.name] = metric.result
				regressionResults[baselineResult.Label].Deltas[metric.name] = calculateDeltaPercent(metric.baseline, metric.result)
			}
		}
	}

	var results = make([]DeltaResult, 0, len(regressionResults)+1)
	for _, result := range regressionResults {
		results = append(results, *result)
	}

	file, err := os.Create(v.DeltaResultsFile)
	if err != nil {
		return errors.Wrap(err, "failed to create delta results file: "+v.DeltaResultsFile)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(results)
	if err != nil {
		return errors.Wrap(err, "failed to encode delta results")
	}

	return nil
}

func (v *GetNetworkDeltaResults) Stop() error {
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

func calculateDeltaPercent(baseline, result float64) float64 {
	if baseline == 0 {
		return 100
	}
	return ((result - baseline) / baseline) * 100
}
