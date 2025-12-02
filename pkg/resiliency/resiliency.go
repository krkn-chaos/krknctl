package resiliency

// Package resiliency centralises all logic related to parsing and aggregating
// resiliency reports that are emitted by the krkn engine.  A report is printed
// on a single log line prefixed by the token `KRKN_RESILIENCY_REPORT_JSON:`
// and contains a JSON payload describing the outcome of one(for krknctl) or more chaos
// scenarios executed by the engine.
//
// The goal of this package is to provide:
//   1.  A small set of structs mirroring the JSON schema.
//   2.  A parser capable of extracting and unmarshalling the payload from a
//       raw block of log text.
//   3.  Helper utilities to merge many individual reports into a single final
//       summary which is later persisted to `resiliency-report.json` by the
//       CLI.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"

	"github.com/krkn-chaos/krknctl/pkg/config"
)

// ----------------------------------------------------------------------------
//  Data structures
// ----------------------------------------------------------------------------

type OverallResiliencyReport struct {
	Scenarios       map[string]float64 `json:"scenarios"`
	ResiliencyScore float64            `json:"resiliency_score"`
	PassedSlos      int                `json:"passed_slos"`
	TotalSlos       int                `json:"total_slos"`
}

// ----------------------------------------------------------------------------

type DetailedScenarioReport struct {
	OverallReport OverallResiliencyReport
	ScenarioWeights map[string]float64 `json:"scenario_weights,omitempty"`
}

// ----------------------------------------------------------------------------

type FinalReport struct {
	Scenarios       map[string]float64 `json:"scenarios"`
	ResiliencyScore float64            `json:"resiliency_score"`
	PassedSlos      int                `json:"passed_slos"`
	TotalSlos       int                `json:"total_slos"`
}

// ----------------------------------------------------------------------------
//  Parser & Aggregator
// ----------------------------------------------------------------------------

var (
	reportRegex *regexp.Regexp
	regexOnce   sync.Once
)

func getReportRegex() *regexp.Regexp {
	regexOnce.Do(func() {
		pattern := `KRKN_RESILIENCY_REPORT_JSON:\s*(\{.*)`
		if cfg, err := config.LoadConfig(); err == nil && cfg.ResiliencyReportRegex != "" {
			pattern = cfg.ResiliencyReportRegex
		}
		reportRegex = regexp.MustCompile(pattern)
	})
	return reportRegex
}

// ParseResiliencyReport searches the supplied log bytes for a line prefixed by
// the special token and, if found, attempts to unmarshal the trailing JSON into
// a DetailedScenarioReport.
func ParseResiliencyReport(logContent []byte) (*DetailedScenarioReport, error) {
	match := getReportRegex().FindSubmatch(logContent)
	if len(match) < 2 {
		return nil, errors.New("resiliency report marker not found in logs")
	}

	raw := match[1]

	var rep DetailedScenarioReport

	// 1. Direct overall_resiliency_report at root.
	type directRoot struct {
		Overall OverallResiliencyReport `json:"overall_resiliency_report"`
	}
	var direct directRoot
	if err := json.Unmarshal(raw, &direct); err == nil && direct.Overall.ResiliencyScore != 0 {
		rep.OverallReport = direct.Overall
		return &rep, nil
	}

	// 2. Nested under telemetry.overall_resiliency_report.
	type telemetryRoot struct {
		Telemetry struct {
			Overall OverallResiliencyReport `json:"overall_resiliency_report"`
		} `json:"telemetry"`
	}
	var telemetry telemetryRoot
	if err := json.Unmarshal(raw, &telemetry); err == nil && telemetry.Telemetry.Overall.ResiliencyScore != 0 {
		rep.OverallReport = telemetry.Telemetry.Overall
		return &rep, nil
	}

	// 3. As a map of scenario scores at root with optional aggregate values.
	type mapRoot struct {
		Scenarios       map[string]float64 `json:"scenarios"`
		ResiliencyScore float64            `json:"resiliency_score"`
		PassedSlos      int                `json:"passed_slos"`
		TotalSlos       int                `json:"total_slos"`
	}
	var mRoot mapRoot
	if err := json.Unmarshal(raw, &mRoot); err == nil && len(mRoot.Scenarios) > 0 {
		rep.OverallReport = OverallResiliencyReport(mRoot)
		return &rep, nil
	}

	// 4. scenarios as an array of objects with name+score.
	type scenarioItem struct {
		Name      string  `json:"name"`
		Score     float64 `json:"score"`
		Breakdown struct {
			Passed int `json:"passed"`
			Failed int `json:"failed"`
		} `json:"breakdown"`
	}
	type arrayRoot struct {
		Scenarios []scenarioItem `json:"scenarios"`
	}
	var aRoot arrayRoot
	if err := json.Unmarshal(raw, &aRoot); err == nil && len(aRoot.Scenarios) > 0 {
		m := make(map[string]float64)
		var total float64
		var passed, totalSLOs int
		for _, it := range aRoot.Scenarios {
			m[it.Name] = it.Score
			total += it.Score
			passed += it.Breakdown.Passed
			totalSLOs += it.Breakdown.Passed + it.Breakdown.Failed
		}
		avg := total / float64(len(aRoot.Scenarios))
		rep.OverallReport = OverallResiliencyReport{
			Scenarios:       m,
			ResiliencyScore: avg,
			PassedSlos:      passed,
			TotalSlos:       totalSLOs,
		}
		return &rep, nil
	}

	return nil, errors.New("unrecognised resiliency report json structure")
}

func AggregateReports(reports []DetailedScenarioReport) FinalReport {
	final := FinalReport{
		Scenarios: make(map[string]float64),
	}

	var weightedSum float64
	var totalWeight float64

	for _, rep := range reports {
		for name, score := range rep.OverallReport.Scenarios {
			final.Scenarios[name] = score

			// Resolve weight – defaults to 1 when absent/invalid.
			weight := 1.0
			if rep.ScenarioWeights != nil {
				if w, ok := rep.ScenarioWeights[name]; ok && w > 0 {
					weight = w
				}
			}

			weightedSum += score * weight
			totalWeight += weight
		}

		// Aggregate SLO counters.
		final.PassedSlos += rep.OverallReport.PassedSlos
		final.TotalSlos += rep.OverallReport.TotalSlos
	}

	if totalWeight > 0 {
		final.ResiliencyScore = weightedSum / totalWeight
	} else {
		final.ResiliencyScore = 100.0
	}

	return final
}

func WriteFinalReport(report FinalReport, path string) error {
	return GenerateAndWriteReport([]DetailedScenarioReport{}, path)
}

// GenerateAndWriteReport generates a resiliency report and writes it to a file
func GenerateAndWriteReport(reports []DetailedScenarioReport, outputPath string) error {
	final := AggregateReports(reports)

	PrintHumanSummary(final)

	type CombinedReport struct {
		Summary FinalReport              `json:"summary"`
		Details []DetailedScenarioReport `json:"details"`
	}

	comb := CombinedReport{
		Summary: final,
		Details: reports,
	}

	data, err := json.MarshalIndent(comb, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write report to %s: %w", outputPath, err)
	}

	return nil
}

func PrintHumanSummary(report FinalReport) {
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		fmt.Printf("overall resiliency report summary:\n%s\n", string(data))
	}
}

// ComputeResiliencyMode returns the value to set in RESILIENCY_ENABLED_MODE.
func ComputeResiliencyMode(prometheusURL string, cfg config.Config) string {
	if prometheusURL != "" {
		return cfg.ResiliencyEnabledMode
	}
	return "disabled"
}
