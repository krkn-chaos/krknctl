package cmd

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestNewEnvironmentTable(t *testing.T) {
	config := getConfig(t)
	longString := "lkaslakslakslakslakslakslakslaksalsklaskaskl"
	values := map[string]ParsedField{"value": {longString, false}}
	var buf bytes.Buffer
	writer := io.Writer(&buf)
	table := NewEnvironmentTable(values, config)
	table = table.WithWriter(writer)
	table.Print()
	assert.NotNil(t, buf)
	stringBuffer := buf.String()
	assert.Contains(t, stringBuffer, fmt.Sprintf("%s...(%d bytes more)", longString[0:config.TableFieldMaxLength], len(longString)-config.TableFieldMaxLength))

	buf.Reset()
	shortString := longString[0:config.TableFieldMaxLength]
	values = map[string]ParsedField{"value": {shortString, false}}
	table = NewEnvironmentTable(values, config)
	table = table.WithWriter(writer)
	table.Print()
	assert.NotNil(t, buf)
	stringBuffer = buf.String()
	assert.NotContains(t, stringBuffer, fmt.Sprintf("%s...(%d bytes more)", longString[0:config.TableFieldMaxLength], len(longString)-config.TableFieldMaxLength))
	assert.Contains(t, stringBuffer, shortString)
}

func TestNewGraphTable(t *testing.T) {
	graph := [][]string{
		{
			"application-outages-test-1",
			"application-outages-test-2",
			"application-outages-test-3",
			"application-outages-test-4",
			"application-outages-test-5",
			"application-outages-test-6",
		},
		{
			"node-cpu-hog-test-1",
			"node-cpu-hog-test-2",
			"node-cpu-hog-test-3",
			"node-cpu-hog-test-4",
			"node-cpu-hog-test-5",
			"node-cpu-hog-test-6",
			"node-cpu-hog-test-7",
			"node-cpu-hog-test-8",
		},
		{
			"do-not-exist-node-cpu-hog-test-1",
			"do-not-exist-node-cpu-hog-test-2",
			"do-not-exist-node-cpu-hog-test-3",
			"do-not-exist-node-cpu-hog-test-4",
			"do-not-exist-node-cpu-hog-test-5",
			"do-not-exist-node-cpu-hog-test-6",
			"do-not-exist-node-cpu-hog-test-7",
			"do-not-exist-node-cpu-hog-test-8",
			"example-1",
			"example-2",
		},

		{
			"affected",
			"africa",
			"alternative",
			"alone",
			"alias",
			"afraid",
			"although",
			"always",
		},
	}

	config := getConfig(t)

	table, err := NewGraphTable(graph, config)
	assert.Nil(t, err)
	var buf bytes.Buffer
	writer := io.Writer(&buf)
	table = table.WithWriter(writer)
	table.Print()
	assert.NotNil(t, buf)
	stringBuffer := buf.String()
	assert.Contains(t, stringBuffer, "(8) node-cpu-hog-test")
	assert.Contains(t, stringBuffer, "(8) do-not-exist-node-cpu-hog-test")
	assert.Contains(t, stringBuffer, "(2) example")
	assert.Contains(t, stringBuffer, "(3) af")
	assert.Contains(t, stringBuffer, "(5) a")

}
