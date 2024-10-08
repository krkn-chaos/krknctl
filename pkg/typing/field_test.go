package typing

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNumberField(t *testing.T) {
	// default value test
	var field Field
	var value *string
	var err error
	var param = ""

	numberFieldDefault := `
	   {
		"name":"cores",
	   	"shortDescription":"Number of cores",
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
	field = Field{}

	// non default value
	numberFieldValue := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
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

	// non default with nil value
	value, err = field.Validate(nil)
	assert.Error(t, err)

	// wrong format
	param = "test"
	value, err = field.Validate(&param)
	assert.Error(t, err)

	param = "2,5"
	value, err = field.Validate(&param)
	assert.Error(t, err)

	// reset
	field = Field{}

	// wrong default, nil default passed
	numberFieldValueWrongDefault := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"number",
	"default":"imwrong"
}
`
	err = json.Unmarshal([]byte(numberFieldValueWrongDefault), &field)
	assert.Nil(t, err)
	param = "100"
	value, err = field.Validate(nil)
	assert.NotNil(t, err)

}

func TestStringField(t *testing.T) {
	var field Field
	var value *string
	var err error
	var param = ""
	stringFieldDefault := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
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
	"shortDescription":"Number of cores",
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
	value, err = field.Validate(&param)
	assert.NotNil(t, err)

	//wrong default
	value, err = field.Validate(nil)
	assert.NotNil(t, err)

}

func TestBooleanField(t *testing.T) {
	var field Field
	var value *string
	var err error
	var param = ""
	booleanFieldDefault := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"boolean",
	"default":"true"
}
`
	err = json.Unmarshal([]byte(booleanFieldDefault), &field)
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
	field = Field{}

	//wrong default
	booleanFieldWrongDefault := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"boolean",
	"default":"imwrong"
}
`
	err = json.Unmarshal([]byte(booleanFieldWrongDefault), &field)
	value, err = field.Validate(nil)
	assert.NotNil(t, err)

}

func TestEnumField(t *testing.T) {
	var field Field
	var value *string
	var err error
	var param = ""
	enumFieldSeparator := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowedValues":"param_1;param_2;param_3",
	"separator":";"
}
`
	err = json.Unmarshal([]byte(enumFieldSeparator), &field)
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
	value, err = field.Validate(&param)
	assert.NotNil(t, err)

	// reset
	field = Field{}

	// not setting the separator defaults to `,`
	enumFieldDefaultSeparator := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowedValues":"param_1,param_2,param_3",
}
`
	err = json.Unmarshal([]byte(enumFieldDefaultSeparator), &field)
	param = "param_1"
	value, err = field.Validate(&param)
	assert.Nil(t, err)
	assert.Equal(t, param, *value)

	field = Field{}

	// setting wrong separator
	enumFieldWrongSeparator := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowedValues":"param_1,param_2,param_3",
	"separator":";"
}
`
	err = json.Unmarshal([]byte(enumFieldWrongSeparator), &field)
	param = "param_1"
	value, err = field.Validate(&param)
	assert.NotNil(t, err)

	field = Field{}

	// setting wrong separator
	enumFieldDefault := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowedValues":"param_1,param_2,param_3",
	"default":"param_2"
}
`
	err = json.Unmarshal([]byte(enumFieldDefault), &field)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.Equal(t, "param_2", *value)

	field = Field{}

	// setting wrong separator
	enumFieldNilValue := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowedValues":"param_1,param_2,param_3"
}
`
	err = json.Unmarshal([]byte(enumFieldNilValue), &field)
	value, err = field.Validate(nil)
	assert.NotNil(t, err)

	field = Field{}

	// setting wrong separator
	enumFieldWrongDefautl := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"enum",
	"allowedValues":"param_1,param_2,param_3",
	"default":"param_4"
}
`
	err = json.Unmarshal([]byte(enumFieldWrongDefautl), &field)
	value, err = field.Validate(nil)
	assert.NotNil(t, err)
}

func TestFieldFile(t *testing.T) {
	var field Field
	var value *string
	var err error
	//var param = ""
	fileField := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"file",
	"mountPath":"/root/.kube/config"
}
`

	fileName := "/tmp/okFile"
	const fileSize = 1 * 1024 * 1024 // 10 MB
	data := make([]byte, fileSize)
	for i := 0; i < fileSize; i++ {
		data[i] = 'A' // Riempie il buffer con il carattere 'A'
	}

	bigfileName := "/tmp/bigFile"
	const bigFileSize = 11 * 1024 * 1024 // 10 MB
	bigData := make([]byte, bigFileSize)
	for i := 0; i < bigFileSize; i++ {
		bigData[i] = 'A' // Riempie il buffer con il carattere 'A'
	}

	err = os.WriteFile(fileName, data, 0644)
	err = os.WriteFile(bigfileName, bigData, 0644)
	defer os.Remove(fileName)
	defer os.Remove(bigfileName)
	assert.Nil(t, err)

	// ok filename
	err = json.Unmarshal([]byte(fileField), &field)
	value, err = field.Validate(&fileName)
	assert.Nil(t, err)
	assert.NotNil(t, value)

	// too big filename
	_, err = field.Validate(&bigfileName)
	assert.NotNil(t, err)

	// no default
	_, err = field.Validate(nil)
	assert.NotNil(t, err)

	field = Field{}

	// file field default
	fileFieldDefault := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"file",
	"mountPath":"/root/.kube/config",
	"default":"/tmp/okFile"
}
`
	err = json.Unmarshal([]byte(fileFieldDefault), &field)
	value, err = field.Validate(nil)
	assert.Nil(t, err)
	assert.NotNil(t, value)

	field = Field{}

	fileFieldNoMountPath := `
{
	"name":"cores",
	"shortDescription":"Number of cores",
	"description":"Number of cores (workers) of node CPU to be consumed",
	"variable":"NODE_CPU_CORE",
	"type":"file"
}
`
	err = json.Unmarshal([]byte(fileFieldNoMountPath), &field)
	_, err = field.Validate(&fileName)
	assert.NotNil(t, err)

}
