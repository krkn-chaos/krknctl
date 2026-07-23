package typing

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNumberField(t *testing.T) {
	// default value test
	var field InputField
	var value *string
	var err error
	var param = ""

	numberFieldDefault := `
	   {
		"name":"cores",
	   	"short_description":"Number of cores",
	   	"description":"Number of cores (workers) of node CPU to be consumed",
	   	"variable":"NODE_CPU_CORE",
	   	"type":"number",
	   	"default":"2"
	   }
	   `
	// number with default
	err = json.Unmarshal([]byte(numberFieldDefault), &field)
	assert.Nil(t, err)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Equal(t, "2", *value)

	// default override
	param = "10.5"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, param, *value)

	// reset
	field = InputField{}

	// non default value
	numberFieldValue := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"number"
}
`
	err = json.Unmarshal([]byte(numberFieldValue), &field)
	assert.Nil(t, err)
	param = "100"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, "100", *value)

	// non default with nil value (required false)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Nil(t, value)

	// wrong format
	param = "test"
	_, err = field.Validate(&param)
	assert.Error(t, err)

	param = "2,5"
	_, err = field.Validate(&param)
	assert.Error(t, err)

	// reset
	field = InputField{}

	// wrong default, nil default passed
	numberFieldValueWrongDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"number",
	"default":"imwrong"
}
`
	err = json.Unmarshal([]byte(numberFieldValueWrongDefault), &field)
	assert.Nil(t, err)
	_, err = field.Validate(nil)
	assert.NotNil(t, err)

	field = InputField{}

	numberRequiredNoDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"number",
	"required": "true"
}
`
	err = json.Unmarshal([]byte(numberRequiredNoDefault), &field)
	assert.Nil(t, err)
	_, err = field.Validate(nil)
	assert.NotNil(t, err)
	param = ""
	_, err = field.Validate(&param)
	assert.NotNil(t, err)

}

func TestStringField(t *testing.T) {
	var field InputField
	var value *string
	var err error
	var param = ""
	stringFieldDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"string",
	"default":"default"
}
`
	err = json.Unmarshal([]byte(stringFieldDefault), &field)
	assert.Nil(t, err)
	// empty string
	param = ""
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, "", *value)

	// default string
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Equal(t, "default", *value)

	stringFieldValidator := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"string",
	"default":"default",
	"validator":"^\".*\"@\"[0-9]+\"",
    "validation_message":"string must be in the format test@1234"
}
`
	err = json.Unmarshal([]byte(stringFieldValidator), &field)
	assert.Nil(t, err)
	// validated string
	param = "\"krkn\"@\"1234\""
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, "\"krkn\"@\"1234\"", *value)

	// not validated + test validation message
	param = "\"krkn\"@\"notvalid\""
	_, err = field.Validate(&param)
	assert.NotNil(t, err)
	assert.Equal(t, "string must be in the format test@1234", err.Error())

	//wrong default
	_, err = field.Validate(nil)
	assert.NotNil(t, err)

	// test validation message without explicit message definition
	stringFieldValidatorNomessage := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"string",
	"default":"default",
	"validator":"^\".*\"@\"[0-9]+\""
}
`
	// not validated
	field = InputField{}
	err = json.Unmarshal([]byte(stringFieldValidatorNomessage), &field)
	assert.Nil(t, err)
	param = "\"krkn\"@\"notvalid\""
	_, err = field.Validate(&param)
	assert.NotNil(t, err)
	assert.Equal(t, "`value`: '\"krkn\"@\"notvalid\"' does not match `validator`: '^\"."+
		"*\"@\"[0-9]+\"'",
		err.Error())

	field = InputField{}

	stringRequiredNoDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"string",
	"required": "true"
}
`
	err = json.Unmarshal([]byte(stringRequiredNoDefault), &field)
	assert.Nil(t, err)
	_, err = field.Validate(nil)
	assert.NotNil(t, err)

	// tests the correctness of a complex string with lots of escapes as a default value

	stringComplexRegex := `
  {
    "name": "telemetry-filter-pattern",
    "short_description": "Telemetry filter pattern",
    "description": "Filter pattern for telemetry logs",
    "variable": "TELEMETRY_FILTER_PATTERN",
    "type": "string",
    "default": "[\"(\\\\w{3}\\\\s\\\\d{1,2}\\\\s\\\\d{2}:\\\\d{2}:\\\\d{2}\\\\.\\\\d+).+\",\"kinit (\\\\d+/\\\\d+/\\\\d+\\\\s\\\\d{2}:\\\\d{2}:\\\\d{2})\\\\s+\",\"(\\\\d{4}-\\\\d{2}-\\\\d{2}T\\\\d{2}:\\\\d{2}:\\\\d{2}\\\\.\\\\d+Z).+\"]",
    "required": "false"
  }
`
	err = json.Unmarshal([]byte(stringComplexRegex), &field)
	assert.Nil(t, err)

}

func TestBooleanField(t *testing.T) {
	var field InputField
	var value *string
	var err error
	var param = ""
	booleanFieldDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"boolean",
	"default":"true"
}
`
	err = json.Unmarshal([]byte(booleanFieldDefault), &field)
	assert.Nil(t, err)
	param = "false"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, param, *value)

	// default
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	param = "true"
	assert.Equal(t, param, *value)

	// reset
	field = InputField{}

	//wrong default
	booleanFieldWrongDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"boolean",
	"default":"imwrong",
	"required": "true"
}
`
	err = json.Unmarshal([]byte(booleanFieldWrongDefault), &field)
	assert.Nil(t, err)
	_, err = field.Validate(nil)
	assert.NotNil(t, err)

	field = InputField{}

	booleanFieldRequiredNoDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"boolean",
	"required": "true"
}
`
	err = json.Unmarshal([]byte(booleanFieldRequiredNoDefault), &field)
	assert.Nil(t, err)
	_, err = field.Validate(nil)
	assert.NotNil(t, err)
	param = ""
	_, err = field.Validate(&param)
	assert.NotNil(t, err)

}

func TestEnumField(t *testing.T) {
	var field InputField
	var value *string
	var err error
	var param = ""
	enumFieldSeparator := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowed_values":"param_1;param_2;param_3",
	"separator":";"
}
`
	err = json.Unmarshal([]byte(enumFieldSeparator), &field)
	assert.Nil(t, err)
	param = "param_1"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, param, *value)

	param = "param_2"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, param, *value)

	param = "param_3"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, param, *value)

	param = "param_4"
	_, err = field.Validate(&param)
	assert.NotNil(t, err)

	// reset
	field = InputField{}

	// not setting the separator defaults to `,`
	enumFieldDefaultSeparator := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowed_values":"param_1,param_2,param_3"
}
`
	err = json.Unmarshal([]byte(enumFieldDefaultSeparator), &field)
	assert.Nil(t, err)
	param = "param_1"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, param, *value)

	field = InputField{}

	// setting wrong separator
	enumFieldWrongSeparator := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowed_values":"param_1,param_2,param_3",
	"separator":";"
}
`
	err = json.Unmarshal([]byte(enumFieldWrongSeparator), &field)
	assert.Nil(t, err)
	param = "param_1"
	_, err = field.Validate(&param)
	assert.NotNil(t, err)

	field = InputField{}

	// setting wrong separator
	enumFieldDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowed_values":"param_1,param_2,param_3",
	"default":"param_2",
	"required": "true"
}
`
	err = json.Unmarshal([]byte(enumFieldDefault), &field)
	assert.Nil(t, err)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Equal(t, "param_2", *value)

	field = InputField{}

	// setting wrong separator
	enumFieldNilValue := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowed_values":"param_1,param_2,param_3"
}
`
	err = json.Unmarshal([]byte(enumFieldNilValue), &field)
	assert.Nil(t, err)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Nil(t, value)

	field = InputField{}

	// setting wrong separator
	enumFieldWrongDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowed_values":"param_1,param_2,param_3",
	"default":"param_4"
}
`
	err = json.Unmarshal([]byte(enumFieldWrongDefault), &field)
	assert.Nil(t, err)
	_, err = field.Validate(nil)
	assert.NotNil(t, err)

	field = InputField{}

	enumFieldRequiredNoDefault := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowed_values":"param_1,param_2,param_3",
	"required":"true"
}
`
	err = json.Unmarshal([]byte(enumFieldRequiredNoDefault), &field)
	assert.Nil(t, err)
	_, err = field.Validate(nil)
	assert.NotNil(t, err)
	param = ""
	_, err = field.Validate(&param)
	assert.NotNil(t, err)
}

func TestFieldFileBase64(t *testing.T) {
	var field InputField
	var value *string
	var err error
	//var param = ""
	fileField := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"file_base64"
}
`

	fileName := fmt.Sprintf("/tmp/okFile.%d", time.Now().Unix())
	const fileSize = 1 * 1024 * 1024 // 1 MB
	data := make([]byte, fileSize)
	for i := 0; i < fileSize; i++ {
		data[i] = 'A'
	}

	bigfileName := fmt.Sprintf("/tmp/bigFile.%d", time.Now().Unix())
	const bigFileSize = 11 * 1024 * 1024 // 11 MB
	bigData := make([]byte, bigFileSize)
	for i := 0; i < bigFileSize; i++ {
		bigData[i] = 'A'
	}

	err = os.WriteFile(fileName, data, 0644)
	assert.Nil(t, err)
	err = os.WriteFile(bigfileName, bigData, 0644)
	assert.Nil(t, err)
	defer os.Remove(fileName)
	defer os.Remove(bigfileName)

	// ok filename
	err = json.Unmarshal([]byte(fileField), &field)
	assert.Nil(t, err)
	value, err = field.Validate(&fileName)
	assert.Nil(t, err)
	assert.NotNil(t, value)

	// too big filename
	_, err = field.Validate(&bigfileName)
	assert.NotNil(t, err)

	// no default
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Nil(t, value)

	// not existent file
	fileNameDoNotExist := "/tmp/donotexist"
	_, err = field.Validate(&fileNameDoNotExist)
	assert.NotNil(t, err)

	// not accessible file
	fileNameNotAccessible := "/etc/shadow"
	_, err = field.Validate(&fileNameNotAccessible)
	assert.NotNil(t, err)

	field = InputField{}

	// file field default
	fileFieldDefault := fmt.Sprintf(`
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"file_base64",
	"default":"%s"
}
`, fileName)
	err = json.Unmarshal([]byte(fileFieldDefault), &field)
	assert.Nil(t, err)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.NotNil(t, value)

}

func TestFieldFile(t *testing.T) {
	var field InputField
	var value *string
	var err error
	//var param = ""
	fileField := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"file",
	"mount_path":"/test/mountpath"
}
`

	fileName := fmt.Sprintf("/tmp/okFile.%d", time.Now().Unix())
	const fileSize = 1 * 1024 * 1024 // 1 MB
	data := make([]byte, fileSize)
	for i := 0; i < fileSize; i++ {
		data[i] = 'A'
	}
	err = os.WriteFile(fileName, data, 0644)
	assert.Nil(t, err)
	defer os.Remove(fileName)

	err = json.Unmarshal([]byte(fileField), &field)
	assert.Nil(t, err)

	// ok file
	value, err = field.Validate(&fileName)
	assert.Nil(t, err)
	assert.NotNil(t, value)
	assert.NotNil(t, field.MountPath)
	assert.Equal(t, "/test/mountpath", *field.MountPath)

	// not existent file
	fileNameDoNotExist := "/tmp/donotexist"
	_, err = field.Validate(&fileNameDoNotExist)
	assert.NotNil(t, err)

	// not accessible file
	fileNameNotAccessible := "/etc/shadow"
	_, err = field.Validate(&fileNameNotAccessible)
	assert.NotNil(t, err)

	// no mount path
	field = InputField{}

	fileFieldNoMountPath := `
{
	"name":"cores",
	"short_description":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"file"
}
`
	json.Unmarshal([]byte(fileFieldNoMountPath), &field)
	_, err = field.Validate(&fileFieldNoMountPath)
	assert.NotNil(t, err)
}

func TestGroupField(t *testing.T) {
	var field InputField
	var err error

	fieldWithGroup := `
{
	"name": "prometheus-url",
	"short_description": "Prometheus url",
	"description": "Prometheus url for when running on kubernetes",
	"variable": "PROMETHEUS_URL",
	"type": "string",
	"default": "",
	"required": "false",
	"group": "prometheus"
}
`
	err = json.Unmarshal([]byte(fieldWithGroup), &field)
	assert.Nil(t, err)
	assert.NotNil(t, field.Group)
	assert.Equal(t, "prometheus", *field.Group)

	// marshal preserves group
	data, err := json.Marshal(&field)
	assert.Nil(t, err)
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	assert.Equal(t, "prometheus", result["group"])

	// round-trip preserves group
	var field2 InputField
	err = json.Unmarshal(data, &field2)
	assert.Nil(t, err)
	assert.NotNil(t, field2.Group)
	assert.Equal(t, *field.Group, *field2.Group)

	// group is optional — field without group parses fine and group is nil
	field = InputField{}
	fieldWithoutGroup := `
{
	"name": "wait-duration",
	"short_description": "Post chaos wait duration",
	"description": "waits for a certain amount of time after the scenario",
	"variable": "WAIT_DURATION",
	"type": "number",
	"default": "1"
}
`
	err = json.Unmarshal([]byte(fieldWithoutGroup), &field)
	assert.Nil(t, err)
	assert.Nil(t, field.Group)

	// group is omitted from marshaled output when nil
	data, err = json.Marshal(&field)
	assert.Nil(t, err)
	result = map[string]interface{}{}
	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	_, groupPresent := result["group"]
	assert.False(t, groupPresent)

	// all known group values round-trip correctly
	knownGroups := []string{"general", "prometheus", "elasticsearch", "cerberus", "telemetry", "health_check", "kubevirt", "resiliency"}
	baseJSON := `{"name":"field","variable":"VAR","type":"string","group":"%s"}`
	for _, group := range knownGroups {
		field = InputField{}
		err = json.Unmarshal([]byte(fmt.Sprintf(baseJSON, group)), &field)
		assert.Nil(t, err, "group %q failed to unmarshal", group)
		assert.NotNil(t, field.Group)
		assert.Equal(t, group, *field.Group)

		data, err = json.Marshal(&field)
		assert.Nil(t, err)
		result = map[string]interface{}{}
		err = json.Unmarshal(data, &result)
		assert.Nil(t, err)
		assert.Equal(t, group, result["group"])
	}
}

func TestGroupFieldsByGroup(t *testing.T) {
	makeField := func(name, variable, group string) InputField {
		n, v := name, variable
		f := InputField{Name: &n, Variable: &v, Type: String}
		if group != "" {
			g := group
			f.Group = &g
		}
		return f
	}

	// empty input returns empty map
	result := GroupFieldsByGroup([]InputField{})
	assert.Empty(t, result)

	// fields without a group land under the empty string key
	noGroupField := makeField("wait-duration", "WAIT_DURATION", "")
	result = GroupFieldsByGroup([]InputField{noGroupField})
	assert.Len(t, result, 1)
	assert.Len(t, result[""], 1)
	assert.Equal(t, "wait-duration", *result[""][0].Name)

	// fields with a group are keyed by that group
	prometheusField := makeField("prometheus-url", "PROMETHEUS_URL", "prometheus")
	result = GroupFieldsByGroup([]InputField{prometheusField})
	assert.Len(t, result, 1)
	assert.Len(t, result["prometheus"], 1)
	assert.Equal(t, "prometheus-url", *result["prometheus"][0].Name)

	// multiple fields in the same group stay together
	prometheusToken := makeField("prometheus-token", "PROMETHEUS_TOKEN", "prometheus")
	result = GroupFieldsByGroup([]InputField{prometheusField, prometheusToken})
	assert.Len(t, result, 1)
	assert.Len(t, result["prometheus"], 2)

	// fields across multiple groups are organized independently
	esField := makeField("es-server", "ES_SERVER", "elasticsearch")
	telField := makeField("telemetry-enabled", "TELEMETRY_ENABLED", "telemetry")
	generalField := makeField("uuid", "UUID", "general")
	ungroupedField := makeField("wait-duration", "WAIT_DURATION", "")

	allFields := []InputField{prometheusField, prometheusToken, esField, telField, generalField, ungroupedField}
	result = GroupFieldsByGroup(allFields)
	assert.Len(t, result, 5)
	assert.Len(t, result["prometheus"], 2)
	assert.Len(t, result["elasticsearch"], 1)
	assert.Len(t, result["telemetry"], 1)
	assert.Len(t, result["general"], 1)
	assert.Len(t, result[""], 1)

	// order within each group matches insertion order
	assert.Equal(t, "prometheus-url", *result["prometheus"][0].Name)
	assert.Equal(t, "prometheus-token", *result["prometheus"][1].Name)
}

func TestGroupTypeValidation(t *testing.T) {
	var field InputField
	var value *string
	var err error

	// Group type field definition
	groupField := `
{
	"name": "prometheus-config",
	"short_description": "Prometheus Configuration Group",
	"description": "Configuration group for Prometheus monitoring settings including URL, token, and retention",
	"variable": "PROMETHEUS_GROUP",
	"type": "group",
	"default": "{\"title\":\"Prometheus\",\"description\":\"Monitoring configuration\"}"
}
`
	err = json.Unmarshal([]byte(groupField), &field)
	assert.Nil(t, err)
	assert.Equal(t, Group, field.Type)

	// Group type accepts any string value (it's metadata)
	testValue := "{\"title\":\"Test Group\",\"short_desc\":\"Test\"}"
	value, err = field.Validate(&testValue)
	assert.Nil(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, testValue, *value)

	// Group type works with default value
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, "{\"title\":\"Prometheus\",\"description\":\"Monitoring configuration\"}", *value)

	// Group type with empty string falls back to default (standard behavior)
	emptyValue := ""
	value, err = field.Validate(&emptyValue)
	assert.Nil(t, err)
	assert.NotNil(t, value)
	// Empty value triggers default for non-string types
	assert.Equal(t, "{\"title\":\"Prometheus\",\"description\":\"Monitoring configuration\"}", *value)

	// Group type field without default and not required accepts nil
	field = InputField{}
	groupFieldNoDefault := `
{
	"name": "optional-group",
	"variable": "OPTIONAL_GROUP",
	"type": "group",
	"required": "false"
}
`
	err = json.Unmarshal([]byte(groupFieldNoDefault), &field)
	assert.Nil(t, err)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Nil(t, value)

	// Group type marshals correctly
	field = InputField{}
	err = json.Unmarshal([]byte(groupField), &field)
	assert.Nil(t, err)
	data, err := json.Marshal(&field)
	assert.Nil(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	assert.Equal(t, "group", result["type"])
}

func TestMarshalJSON(t *testing.T) {
	name := "test-field"
	variable := "TEST_VAR"
	description := "Test description"
	defaultValue := "default-value"
	separator := ";"
	allowedValues := "val1;val2;val3"
	mountPath := "/test/path"

	// Test with all fields populated
	field := InputField{
		Name:         &name,
		Description:  &description,
		Variable:     &variable,
		Type:         Enum,
		Default:      &defaultValue,
		Separator:    &separator,
		AllowedValues: &allowedValues,
		Required:     true,
		MountPath:    &mountPath,
		Secret:       false,
	}

	data, err := json.Marshal(&field)
	assert.Nil(t, err)
	assert.NotNil(t, data)

	// Unmarshal to map to check string conversion
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)

	// Verify Type is converted to string
	assert.Equal(t, "enum", result["type"])

	// Verify Required is converted to string
	assert.Equal(t, "true", result["required"])

	// Verify Secret is converted to string
	assert.Equal(t, "false", result["secret"])

	// Verify other fields are preserved
	assert.Equal(t, name, result["name"])
	assert.Equal(t, variable, result["variable"])
	assert.Equal(t, description, result["description"])
	assert.Equal(t, defaultValue, result["default"])

	// Test with different Type values
	field.Type = Boolean
	field.Required = false
	field.Secret = true
	data, err = json.Marshal(&field)
	assert.Nil(t, err)

	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	assert.Equal(t, "boolean", result["type"])
	assert.Equal(t, "false", result["required"])
	assert.Equal(t, "true", result["secret"])

	// Test with Number type
	field.Type = Number
	data, err = json.Marshal(&field)
	assert.Nil(t, err)

	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	assert.Equal(t, "number", result["type"])

	// Test with String type
	field.Type = String
	data, err = json.Marshal(&field)
	assert.Nil(t, err)

	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	assert.Equal(t, "string", result["type"])

	// Test with File type
	field.Type = File
	data, err = json.Marshal(&field)
	assert.Nil(t, err)

	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	assert.Equal(t, "file", result["type"])

	// Test with FileBase64 type
	field.Type = FileBase64
	data, err = json.Marshal(&field)
	assert.Nil(t, err)

	err = json.Unmarshal(data, &result)
	assert.Nil(t, err)
	assert.Equal(t, "file_base64", result["type"])

	// Test round-trip: Unmarshal -> Marshal -> Unmarshal
	originalJSON := `{
		"name": "round-trip-test",
		"variable": "ROUND_TRIP_VAR",
		"type": "enum",
		"allowed_values": "a,b,c",
		"required": "true",
		"secret": "false"
	}`

	var field2 InputField
	err = json.Unmarshal([]byte(originalJSON), &field2)
	assert.Nil(t, err)

	marshaledData, err := json.Marshal(&field2)
	assert.Nil(t, err)

	var field3 InputField
	err = json.Unmarshal(marshaledData, &field3)
	assert.Nil(t, err)

	// Verify fields match after round-trip
	assert.Equal(t, field2.Type, field3.Type)
	assert.Equal(t, field2.Required, field3.Required)
	assert.Equal(t, field2.Secret, field3.Secret)
	assert.Equal(t, *field2.Name, *field3.Name)
	assert.Equal(t, *field2.Variable, *field3.Variable)
}
