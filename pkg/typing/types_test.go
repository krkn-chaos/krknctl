package typing

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestIsFile(t *testing.T) {
	f, err := os.Create("testFile.txt")
	assert.Nil(t, err)
	defer f.Close()
	assert.True(t, IsFile("testFile.txt"))
	assert.False(t, IsFile("NoTestFile.txt"))
}

func TestIsEnum(t *testing.T) {
	assert.True(t, IsEnum("test_2", ",", "test_1,test_2,test_3"))
	assert.False(t, IsEnum("test_4", ",", "test_1,test_2,test_3"))
	assert.True(t, IsEnum("test_2", "!", "test_1!test_2!test_3"))
	assert.False(t, IsEnum("test_2", "!", "test_1,test_2,test_3"))
}

func TestIsBoolean(t *testing.T) {
	assert.True(t, IsBoolean("true"))
	assert.True(t, IsBoolean("false"))
	assert.True(t, IsBoolean("True"))
	assert.True(t, IsBoolean("False"))
	assert.False(t, IsBoolean("Falser"))
}

func TestIsNumber(t *testing.T) {
	assert.True(t, IsNumber("1"))
	assert.False(t, IsNumber("TEST"))
	assert.True(t, IsNumber("1.1"))
	assert.False(t, IsNumber("1,2"))
	assert.True(t, IsNumber(".1"))
	assert.True(t, IsNumber("1_000_000"))
}

func TestToType(t *testing.T) {

	krknctlType := String
	assert.Equal(t, krknctlType.String(), "string")
	assert.Equal(t, ToType("string"), krknctlType)

	krknctlType = Boolean
	assert.Equal(t, krknctlType.String(), "boolean")
	assert.Equal(t, ToType("boolean"), krknctlType)

	krknctlType = Number
	assert.Equal(t, krknctlType.String(), "number")
	assert.Equal(t, ToType("number"), krknctlType)

	krknctlType = Enum
	assert.Equal(t, krknctlType.String(), "enum")
	assert.Equal(t, ToType("enum"), krknctlType)

	krknctlType = File
	assert.Equal(t, krknctlType.String(), "file")
	assert.Equal(t, ToType("file"), krknctlType)

	krknctlType = FileBase64
	assert.Equal(t, krknctlType.String(), "file_base64")
	assert.Equal(t, ToType("file_base64"), krknctlType)

	assert.Equal(t, ToType("anything"), Unknown)

}
