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
	"validator":"^\".*\"@\"[0-9]+\""
}
`
	err = json.Unmarshal([]byte(stringFieldValidator), &field)
	assert.Nil(t, err)
	// validated string
	param = "\"krkn\"@\"1234\""
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, "\"krkn\"@\"1234\"", *value)

	// not validated
	param = "\"krkn\"@\"notvalid\""
	_, err = field.Validate(&param)
	assert.NotNil(t, err)

	//wrong default
	_, err = field.Validate(nil)
	assert.NotNil(t, err)

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
