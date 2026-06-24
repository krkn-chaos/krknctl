package resiliency

import (
	"os"
	"testing"

	"github.com/krkn-chaos/krknctl/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestParseResiliencyReport_DirectRoot(t *testing.T) {
	logContent := []byte(`KRKN_RESILIENCY_REPORT_JSON: {"overall_resiliency_report": {"scenarios": {"test-scenario": 95.5}, "resiliency_score": 95.5, "passed_slos": 10, "total_slos": 12}}`)

	rep, err := ParseResiliencyReport(logContent)
	assert.Nil(t, err)
	assert.NotNil(t, rep)
	assert.Equal(t, 95.5, rep.OverallReport.ResiliencyScore)
	assert.Equal(t, 10, rep.OverallReport.PassedSlos)
	assert.Equal(t, 12, rep.OverallReport.TotalSlos)
}

func TestParseResiliencyReport_DirectRootWithZeroScore(t *testing.T) {
	// Verify that reports with resiliency_score=0 are accepted (all tests failed)
	logContent := []byte(`KRKN_RESILIENCY_REPORT_JSON: {"overall_resiliency_report": {"scenarios": {"test-scenario": 0.0}, "resiliency_score": 0.0, "passed_slos": 0, "total_slos": 10}}`)

	rep, err := ParseResiliencyReport(logContent)
	assert.Nil(t, err, "Score of 0.0 should be valid")
	assert.NotNil(t, rep)
	assert.Equal(t, 0.0, rep.OverallReport.ResiliencyScore)
	assert.Equal(t, 0, rep.OverallReport.PassedSlos)
	assert.Equal(t, 10, rep.OverallReport.TotalSlos)
}

func TestParseResiliencyReport_TelemetryNestedWithZeroScore(t *testing.T) {
	// Verify telemetry-nested reports with score=0 are accepted
	logContent := []byte(`KRKN_RESILIENCY_REPORT_JSON: {"telemetry": {"overall_resiliency_report": {"scenarios": {"test-scenario": 0.0}, "resiliency_score": 0.0, "passed_slos": 0, "total_slos": 5}}}`)

	rep, err := ParseResiliencyReport(logContent)
	assert.Nil(t, err, "Telemetry-nested score of 0.0 should be valid")
	assert.NotNil(t, rep)
	assert.Equal(t, 0.0, rep.OverallReport.ResiliencyScore)
	assert.Equal(t, 0, rep.OverallReport.PassedSlos)
	assert.Equal(t, 5, rep.OverallReport.TotalSlos)
}

func TestParseResiliencyReport_TelemetryNested(t *testing.T) {
	logContent := []byte(`KRKN_RESILIENCY_REPORT_JSON: {"telemetry": {"overall_resiliency_report": {"scenarios": {"test-scenario": 88.0}, "resiliency_score": 88.0, "passed_slos": 8, "total_slos": 10}}}`)

	rep, err := ParseResiliencyReport(logContent)
	assert.Nil(t, err)
	assert.NotNil(t, rep)
	assert.Equal(t, 88.0, rep.OverallReport.ResiliencyScore)
}

func TestParseResiliencyReport_MapRoot(t *testing.T) {
	logContent := []byte(`KRKN_RESILIENCY_REPORT_JSON: {"scenarios": {"scenario-a": 90.0, "scenario-b": 85.0}, "resiliency_score": 87.5, "passed_slos": 15, "total_slos": 20}`)
	
	rep, err := ParseResiliencyReport(logContent)
	assert.Nil(t, err)
	assert.NotNil(t, rep)
	assert.Equal(t, 87.5, rep.OverallReport.ResiliencyScore)
	assert.Equal(t, 2, len(rep.OverallReport.Scenarios))
}

func TestParseResiliencyReport_ArrayRoot(t *testing.T) {
	logContent := []byte(`KRKN_RESILIENCY_REPORT_JSON: {"scenarios": [{"name": "scenario-1", "score": 92.0, "weight": 1.0}, {"name": "scenario-2", "score": 88.0, "weight": 1.5}]}`)
	
	rep, err := ParseResiliencyReport(logContent)
	assert.Nil(t, err)
	assert.NotNil(t, rep)
	assert.Equal(t, 2, len(rep.OverallReport.Scenarios))
	assert.Equal(t, 2, len(rep.Scenarios))
	assert.Equal(t, "scenario-1", rep.Scenarios[0].Name)
	assert.Equal(t, 92.0, rep.Scenarios[0].Score)
}

func TestParseResiliencyReport_ArrayRootWithBreakdown(t *testing.T) {
	// Test that totalSLOs is correctly calculated as passed + failed, not double-counted
	logContent := []byte(`KRKN_RESILIENCY_REPORT_JSON: {"scenarios": [{"name": "scenario-1", "score": 90.0, "breakdown": {"passed": 9.0, "failed": 1.0}}, {"name": "scenario-2", "score": 80.0, "breakdown": {"passed": 8.0, "failed": 2.0}}]}`)

	rep, err := ParseResiliencyReport(logContent)
	assert.Nil(t, err)
	assert.NotNil(t, rep)
	assert.Equal(t, 2, len(rep.OverallReport.Scenarios))

	// Verify passed/failed counts are correctly summed
	assert.Equal(t, 17, rep.OverallReport.PassedSlos, "PassedSlos should be 9 + 8 = 17")
	assert.Equal(t, 20, rep.OverallReport.TotalSlos, "TotalSlos should be (9+1) + (8+2) = 20, not double-counted")
}

func TestParseResiliencyReport_NotFound(t *testing.T) {
	logContent := []byte(`Some random log output without the marker`)

	rep, err := ParseResiliencyReport(logContent)
	assert.NotNil(t, err)
	assert.Nil(t, rep)
	assert.Contains(t, err.Error(), "resiliency report marker not found")
}

func TestAggregateReports_SingleReport(t *testing.T) {
	reports := []DetailedScenarioReport{
		{
			OverallReport: OverallResiliencyReport{
				Scenarios:       map[string]float64{"test": 90.0},
				ResiliencyScore: 90.0,
				PassedSlos:      9,
				TotalSlos:       10,
			},
		},
	}
	
	final := AggregateReports(reports)
	assert.Equal(t, 90.0, final.ResiliencyScore)
	assert.Equal(t, 9, final.PassedSlos)
	assert.Equal(t, 10, final.TotalSlos)
}

func TestAggregateReports_MultipleReports(t *testing.T) {
	reports := []DetailedScenarioReport{
		{
			OverallReport: OverallResiliencyReport{
				Scenarios:       map[string]float64{"scenario-a": 80.0},
				ResiliencyScore: 80.0,
				PassedSlos:      8,
				TotalSlos:       10,
			},
		},
		{
			OverallReport: OverallResiliencyReport{
				Scenarios:       map[string]float64{"scenario-b": 90.0},
				ResiliencyScore: 90.0,
				PassedSlos:      9,
				TotalSlos:       10,
			},
		},
	}
	
	final := AggregateReports(reports)
	assert.Equal(t, 85.0, final.ResiliencyScore)
	assert.Equal(t, 17, final.PassedSlos)
	assert.Equal(t, 20, final.TotalSlos)
	assert.Equal(t, 2, len(final.Scenarios))
}

func TestAggregateReports_WithWeights(t *testing.T) {
	reports := []DetailedScenarioReport{
		{
			OverallReport: OverallResiliencyReport{
				Scenarios:       map[string]float64{"scenario-a": 80.0},
				ResiliencyScore: 80.0,
			},
			ScenarioWeights: map[string]float64{"scenario-a": 2.0},
		},
		{
			OverallReport: OverallResiliencyReport{
				Scenarios:       map[string]float64{"scenario-b": 90.0},
				ResiliencyScore: 90.0,
			},
			ScenarioWeights: map[string]float64{"scenario-b": 1.0},
		},
	}
	
	final := AggregateReports(reports)
	// (80*2 + 90*1) / (2+1) = 250/3 = 83.33...
	assert.InDelta(t, 83.33, final.ResiliencyScore, 0.01)
}

func TestAggregateReports_EmptyReports(t *testing.T) {
	reports := []DetailedScenarioReport{}
	
	final := AggregateReports(reports)
	assert.Equal(t, 100.0, final.ResiliencyScore)
	assert.Equal(t, 0, final.PassedSlos)
	assert.Equal(t, 0, final.TotalSlos)
}

func TestGenerateAndWriteReport(t *testing.T) {
	reports := []DetailedScenarioReport{
		{
			OverallReport: OverallResiliencyReport{
				Scenarios:       map[string]float64{"test": 85.0},
				ResiliencyScore: 85.0,
				PassedSlos:      8,
				TotalSlos:       10,
			},
		},
	}
	
	tmpFile := t.TempDir() + "/test-report.json"
	err := GenerateAndWriteReport(reports, tmpFile)
	assert.Nil(t, err)
	
	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(tmpFile)
	assert.Nil(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "summary")
	assert.Contains(t, string(data), "details")
}

func TestComputeResiliencyMode(t *testing.T) {
	cfg := config.Config{
		ResiliencyEnabledMode: "enabled",
	}
	
	mode := ComputeResiliencyMode("http://prometheus:9090", cfg)
	assert.Equal(t, "enabled", mode)
	
	mode = ComputeResiliencyMode("", cfg)
	assert.Equal(t, "disabled", mode)
}

