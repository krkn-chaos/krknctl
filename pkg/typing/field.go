package typing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
)

const MAX_FILE_SIZE int64 = 10_485_760

type Field struct {
	Name             *string `json:"name"`
	ShortDescription *string `json:"ShortDescription,omitempty"`
	Description      *string `json:"description,omitempty"`
	Variable         *string `json:"variable"`
	Type             Type    `json:"type"`
	Default          *string `json:"default,omitempty"`
	Validator        *string `json:"validator,omitempty"`
	Separator        *string `json:"separator,omitempty"`
	AllowedValues    *string `json:"allowedValues,omitempty"`
	MountPath        *string `json:"mountPath,omitempty"`
}

type alias Field

func (f *Field) UnmarshalJSON(data []byte) error {
	aux := &struct {
		*alias
		Name     *string `json:"name"`
		Type     *string `json:"type"`
		Variable *string `json:"variable"`
	}{alias: (*alias)(f)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var temp map[string]string
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// REQUIRED VALUES ARE DESERIALIZED MANUALLY
	// Field Type parsing
	if fieldType, ok := temp["type"]; ok {
		f.Type = ToType(fieldType)
		if f.Type == Unknown {
			return errors.New("unknown field type: " + fieldType)
		}
	} else {
		return errors.New("`type` key not found")
	}

	// variable must be always present since represents
	// the envvar to be exported in the container
	if fieldProperty, ok := temp["variable"]; ok {
		f.Variable = &fieldProperty
	} else {
		return errors.New("`variable` key not found")
	}

	if fieldProperty, ok := temp["name"]; ok {
		f.Name = &fieldProperty
	} else {
		return errors.New("`name` key not found")
	}
	return nil
}

func (f *Field) Validate(value *string) (*string, error) {

	if value == nil && f.Default == nil && f.Type == String {
		return nil, errors.New("`" + f.Type.String() + " can be blank, but not null without a default")
	}

	if (value == nil || *value == "") && (f.Default == nil || *f.Default == "") && f.Type != String {
		return nil, errors.New("field `value` doesn't have a `default` and cannot be nil or empty for type `" + f.Type.String() + "`")
	}

	if (value == nil || *value == "") && (f.Default == nil || *f.Default == "") && f.Type != String {
		return nil, errors.New("`" + f.Type.String() + " can't be null without a (non-blank) default")
	}

	var selectedValue *string
	if value == nil || (*value == "" && f.Type != String) {
		selectedValue = f.Default
		// recursive call to validate default value
		// to avoid schema development errors
		if _, err := f.Validate(selectedValue); err != nil {
			return nil, errors.New("schema validation error on default value: " + err.Error())
		}
	} else {
		selectedValue = value
	}

	switch f.Type {
	case String:
		if f.Validator != nil {
			match, err := regexp.MatchString(*f.Validator, *selectedValue)
			if err != nil {
				return nil, err
			}
			if match == false {
				return nil, errors.New("`value`: '" + *selectedValue + "' does not match `validator`: '" + *f.Validator + "'")
			}
		}
	case Number:
		if IsNumber(*selectedValue) == false {
			return nil, errors.New("`value`: '" + *selectedValue + "' is not a number")
		}
	case Boolean:
		if IsBoolean(*selectedValue) == false {
			return nil, errors.New("`value`: '" + *selectedValue + "' is not a boolean")
		}
	case Enum:
		defaultSeparator := ","
		var separator *string
		if f.Separator == nil {
			separator = &defaultSeparator
		} else {
			separator = f.Separator
		}
		if f.AllowedValues == nil {
			return nil, errors.New("invalid schema: `allowedValues` is required for enum types")
		}
		if IsEnum(*selectedValue, *separator, *f.AllowedValues) == false {
			return nil, errors.New("`value`: '" + *selectedValue + "' is not in: '" + *f.AllowedValues + "' separated by: '" + *separator + "'")
		}
	case File:
		if IsFile(*selectedValue) {
			if f.MountPath == nil {
				return nil, errors.New("invalid schema: `mountPath` is required for file types")
			}
			fileInfo, err := os.Stat(*selectedValue)
			if err != nil {
				return nil, err
			}
			if fileInfo.Size() > MAX_FILE_SIZE {
				return nil, fmt.Errorf("`%s` exceeds %d bytes", *selectedValue, MAX_FILE_SIZE)
			}
			file, err := os.Open(*selectedValue)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			var buffer bytes.Buffer
			encoder := base64.NewEncoder(base64.StdEncoding, &buffer)
			defer encoder.Close()
			_, err = io.Copy(encoder, file)
			if err != nil {
				return nil, err
			}
			if err := encoder.Close(); err != nil {
				return nil, err
			}
			encodedData := buffer.String()
			return &encodedData, nil
		}

	default:
		return nil, errors.New("impossible to validate object")
	}
	return selectedValue, nil
}
