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
)

// ----------------------------------------------------------------------------
//  Data structures
// ----------------------------------------------------------------------------

type OverallResiliencyReport struct {
    Scenarios       map[string]float64 `json:"scenarios"`
    ResiliencyScore float64            `json:"resiliency_score"`
    PassedSlos      int               `json:"passed_slos"`
    TotalSlos       int               `json:"total_slos"`
}


// ----------------------------------------------------------------------------


type DetailedScenarioReport struct {
    OverallReport OverallResiliencyReport
}

// ----------------------------------------------------------------------------

type FinalReport struct {
    Scenarios       map[string]float64 `json:"scenarios"`
    ResiliencyScore float64            `json:"resiliency_score"`
    PassedSlos      int               `json:"passed_slos"`
    TotalSlos       int               `json:"total_slos"`
}

// ----------------------------------------------------------------------------
//  Parser & Aggregator
// ----------------------------------------------------------------------------

var reportRegex = regexp.MustCompile(`KRKN_RESILIENCY_REPORT_JSON:\s*(\{.*)`)

// ParseResiliencyReport searches the supplied log bytes for a line prefixed by
// the special token and, if found, attempts to unmarshal the trailing JSON into
// a DetailedScenarioReport.
func ParseResiliencyReport(logContent []byte) (*DetailedScenarioReport, error) {
    match := reportRegex.FindSubmatch(logContent)
    if len(match) < 2 {
        return nil, errors.New("resiliency report marker not found in logs")
    }

    raw := match[1]

    var rep DetailedScenarioReport

    // 1. Direct overall_resiliency_report at root.
    type root1 struct {
        Overall OverallResiliencyReport `json:"overall_resiliency_report"`
    }
    var r1 root1
    if err := json.Unmarshal(raw, &r1); err == nil && r1.Overall.ResiliencyScore != 0 {
        rep.OverallReport = r1.Overall
        return &rep, nil
    }

    // 2. Nested under telemetry.overall_resiliency_report.
    type root2 struct {
        Telemetry struct {
            Overall OverallResiliencyReport `json:"overall_resiliency_report"`
        } `json:"telemetry"`
    }
    var r2 root2
    if err := json.Unmarshal(raw, &r2); err == nil && r2.Telemetry.Overall.ResiliencyScore != 0 {
        rep.OverallReport = r2.Telemetry.Overall
        return &rep, nil
    }

    // 3. As a map of scenario scores at root with optional aggregate values.
    type root3 struct {
        Scenarios        map[string]float64 `json:"scenarios"`
        ResiliencyScore  float64           `json:"resiliency_score"`
        PassedSlos       int               `json:"passed_slos"`
        TotalSlos        int               `json:"total_slos"`
    }
    var r3 root3
    if err := json.Unmarshal(raw, &r3); err == nil && len(r3.Scenarios) > 0 {
        rep.OverallReport = OverallResiliencyReport{
            Scenarios:       r3.Scenarios,
            ResiliencyScore: r3.ResiliencyScore,
            PassedSlos:      r3.PassedSlos,
            TotalSlos:       r3.TotalSlos,
        }
        return &rep, nil
    }

    // 4. scenarios as an array of objects with name+score.
    type scenarioItem struct {
        Name  string  `json:"name"`
        Score float64 `json:"score"`
        Breakdown struct {
            Passed int `json:"passed"`
            Failed int `json:"failed"`
        } `json:"breakdown"`
    }
    type root4 struct {
        Scenarios []scenarioItem `json:"scenarios"`
    }
    var r4 root4
    if err := json.Unmarshal(raw, &r4); err == nil && len(r4.Scenarios) > 0 {
        m := make(map[string]float64)
        var total float64
        var passed, totalSLOs int
        for _, it := range r4.Scenarios {
            m[it.Name] = it.Score
            total += it.Score
            passed += it.Breakdown.Passed
            totalSLOs += it.Breakdown.Passed + it.Breakdown.Failed
        }
        avg := total / float64(len(r4.Scenarios))
        rep.OverallReport = OverallResiliencyReport{
            Scenarios:       m,
            ResiliencyScore: avg,
            PassedSlos:      passed,
            TotalSlos:       totalSLOs,
        }
        return &rep, nil
    }

    return nil, errors.New("unrecognised resiliency report JSON structure")
}

/// TODO: @abhinavs1920 (Implement weighted average of scores) 
func AggregateReports(reports []DetailedScenarioReport) FinalReport {
    final := FinalReport{
        Scenarios: make(map[string]float64),
    }

    var scoreSum float64
    var scoreCount int

    for _, rep := range reports {
        // Merge per-scenario scores.
        for name, score := range rep.OverallReport.Scenarios {
            final.Scenarios[name] = score
            scoreSum += score
            scoreCount++
        }
        // Aggregate SLO counters.
        final.PassedSlos += rep.OverallReport.PassedSlos
        final.TotalSlos += rep.OverallReport.TotalSlos
    }

    if scoreCount > 0 {
        final.ResiliencyScore = scoreSum / float64(scoreCount)
    } else {
        // No data -> perfect score.
        final.ResiliencyScore = 100.0
    }

    return final
}

func WriteFinalReport(report FinalReport, filename string) error {
    data, err := json.MarshalIndent(report, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(filename, data, 0o644)
}

func PrintHumanSummary(report FinalReport) {
    if data, err := json.MarshalIndent(report, "", "  "); err == nil {
        fmt.Printf("Overall Resiliency Report Summary:\n%s\n", string(data))
    }
}
