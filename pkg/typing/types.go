package typing

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Type int64

const (
	String Type = iota
	Boolean
	Number
	Enum
	Unknown
	File
	FileBase64
)

func (t Type) String() string {
	switch t {
	case String:
		return "string"
	case Boolean:
		return "boolean"
	case Number:
		return "number"
	case Enum:
		return "enum"
	case File:
		return "file"
	case FileBase64:
		return "file_base64"
	default:
		return "unknown"
	}
}

func ToType(s string) Type {
	switch strings.ToLower(s) {
	case "string":
		return String
	case "boolean":
		return Boolean
	case "number":
		return Number
	case "enum":
		return Enum
	case "file":
		return File
	case "file_base64":
		return FileBase64
	default:
		return Unknown
	}
}

func IsNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func IsBoolean(s string) bool {
	_, err := strconv.ParseBool(strings.ToLower(s))
	return err == nil
}

func IsEnum(s string, separator string, allowedValues string) bool {
	values := strings.Split(allowedValues, separator)
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}

func IsFile(f string) bool {
	if _, err := os.Stat(f); errors.Is(err, os.ErrNotExist) {
		return false
	}

	file, err := os.Open(f)

	defer func() {
		if err == nil && file != nil {
			deferErr := file.Close()
			if deferErr != nil {
				panic(deferErr)
			}
		}
	}()

	if err != nil {
		return false
	}
	return true
}
