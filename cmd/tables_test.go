package cmd

import (
	"testing"
)

func TestNewEnvironmentTable(t *testing.T) {
	config := getConfig(t)
	values := map[string]ParsedField{"value": {"lkaslakslakslakslakslakslakslaksalsklaskaskl", false}}
	table := NewEnvironmentTable(values, config)
	table.Print()
}
