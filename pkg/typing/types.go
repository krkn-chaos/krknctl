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
	return true
}
