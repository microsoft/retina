package perf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type CPUUtilization struct {
	Host   float64 `json:"host"`
	Remote float64 `json:"remote"`
}

type TestResult struct {
	TotalThroughput float64        `json:"total_throughput"`
	MeanRTT         int            `json:"mean_rtt,omitempty"`
	MinRTT          int            `json:"min_rtt,omitempty"`
	MaxRTT          int            `json:"max_rtt,omitempty"`
	Retransmits     int            `json:"retransmits,omitempty"`
	JitterMs        float64        `json:"jitter_ms,omitempty"`
	LostPackets     int            `json:"lost_packets,omitempty"`
	TotalPackets    int            `json:"total_packets,omitempty"`
	LostPercent     float64        `json:"lost_percent,omitempty"`
	CPUUtilization  CPUUtilization `json:"cpu_utilization"`
}

type TestCase struct {
	Label  string     `json:"label"`
	Result TestResult `json:"result"`
}

type RegressionResult struct {
	Label      string  `json:"label"`
	Metric     string  `json:"metric"`
	BaseValue  float64 `json:"base_value"`
	NewValue   float64 `json:"new_value"`
	Regression float64 `json:"regression"`
}

type AggregatedResult struct {
	Label       string             `json:"label"`
	BaseMetrics map[string]float64 `json:"base_metrics"`
	Metrics     map[string]float64 `json:"metrics"`
	Regressions map[string]float64 `json:"regressions"`
}

type GetNetworkRegressionResults struct {
	BaseResultsFile string
	NewResultsFile  string
}

func (v *GetNetworkRegressionResults) Prevalidate() error {
	return nil
}

func (v *GetNetworkRegressionResults) Run() error {
	testCases1, err := readJSONFile(v.BaseResultsFile)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", v.BaseResultsFile, err)
	}

	testCases2, err := readJSONFile(v.NewResultsFile)
	if err != nil {
		return fmt.Errorf("error reading file %s: %v", v.NewResultsFile, err)
	}

	if len(testCases1) != len(testCases2) {
		return fmt.Errorf("number of test cases in the two files do not match")
	}

	aggregatedResults := make(map[string]*AggregatedResult)

	for i := range testCases1 {
		tc1 := testCases1[i]
		tc2 := testCases2[i]

		if _, exists := aggregatedResults[tc1.Label]; !exists {
			aggregatedResults[tc1.Label] = &AggregatedResult{
				Label:       tc1.Label,
				BaseMetrics: make(map[string]float64),
				Metrics:     make(map[string]float64),
				Regressions: make(map[string]float64),
			}
		}

		metrics := []struct {
			name   string
			oldVal float64
			newVal float64
		}{
			{"Total Throughput", tc1.Result.TotalThroughput, tc2.Result.TotalThroughput},
			{"Mean RTT", float64(tc1.Result.MeanRTT), float64(tc2.Result.MeanRTT)},
			{"Min RTT", float64(tc1.Result.MinRTT), float64(tc2.Result.MinRTT)},
			{"Max RTT", float64(tc1.Result.MaxRTT), float64(tc2.Result.MaxRTT)},
			{"Retransmits", float64(tc1.Result.Retransmits), float64(tc2.Result.Retransmits)},
			{"Jitter (ms)", tc1.Result.JitterMs, tc2.Result.JitterMs},
			{"Lost Packets", float64(tc1.Result.LostPackets), float64(tc2.Result.LostPackets)},
			{"Lost Percent", tc1.Result.LostPercent, tc2.Result.LostPercent},
			{"CPU Utilization Host", tc1.Result.CPUUtilization.Host, tc2.Result.CPUUtilization.Host},
			{"CPU Utilization Remote", tc1.Result.CPUUtilization.Remote, tc2.Result.CPUUtilization.Remote},
		}

		for _, metric := range metrics {
			if metric.oldVal != 0 || metric.newVal != 0 {
				aggregatedResults[tc1.Label].BaseMetrics[metric.name] = metric.oldVal
				aggregatedResults[tc1.Label].Metrics[metric.name] = metric.newVal
				aggregatedResults[tc1.Label].Regressions[metric.name] = calculateRegression(metric.oldVal, metric.newVal)
			}
		}
	}

	var results []AggregatedResult
	for _, result := range aggregatedResults {
		results = append(results, *result)
	}

	outputFile := fmt.Sprintf("network-regression-results-%s.json", time.Now().Format("20060102150405"))
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating file %s: %v", outputFile, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		return fmt.Errorf("error encoding results to JSON: %v", err)
	}

	defer deleteFile(v.BaseResultsFile)
	defer deleteFile(v.NewResultsFile)

	return nil
}

func (v *GetNetworkRegressionResults) Stop() error {
	return nil
}

func readJSONFile(filename string) ([]TestCase, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var testCases []TestCase
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
