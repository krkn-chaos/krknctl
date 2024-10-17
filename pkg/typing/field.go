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
	"strconv"
)

const MaxFileSize int64 = 10_485_760

type InputField struct {
	Name             *string `json:"name"`
	ShortDescription *string `json:"short_description,omitempty"`
	Description      *string `json:"description,omitempty"`
	Variable         *string `json:"variable"`
	Type             Type    `json:"type"`
	Default          *string `json:"default,omitempty"`
	Validator        *string `json:"validator,omitempty"`
	Separator        *string `json:"separator,omitempty"`
	AllowedValues    *string `json:"allowed_values,omitempty"`
	Required         bool    `json:"required,omitempty"`
	MountPath        *string `json:"mount_path,omitempty"`
	Requires         *string `json:"requires,omitempty"`
	MutuallyExcludes *string `json:"mutually_excludes,omitempty"`
}

type alias InputField

func (f *InputField) UnmarshalJSON(data []byte) error {
	aux := &struct {
		*alias
		Name     *string `json:"name"`
		Type     *string `json:"type"`
		Variable *string `json:"variable"`
		Required *string `json:"required"`
	}{alias: (*alias)(f)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var temp map[string]string
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// REQUIRED VALUES ARE DESERIALIZED MANUALLY
	// InputField Type parsing
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

	if fieldProperty, ok := temp["required"]; ok {
		required, err := strconv.ParseBool(fieldProperty)
		if err != nil {
			return err
		}
		f.Required = required
	} else {
		f.Required = false
	}
	return nil
}

func (f *InputField) Validate(value *string) (*string, error) {
	// if string value is nil, the default value is nil and the field is required the field is not valid
	if value == nil && f.Default == nil && f.Required == true && f.Type == String {
		return nil, errors.New("`" + f.Type.String() + " can be blank, but not null without a default if required")
	}
	// if any other type value is nil or empty, the default value is nil or empty and the field is required the field is not valid
	if (value == nil || *value == "") && (f.Default == nil || *f.Default == "") && f.Required == true && f.Type != String {
		return nil, errors.New("field `value` doesn't have a `default` and cannot be nil or empty for type `" + f.Type.String() + "`")
	}

	var selectedValue *string
	// if the default value is not nil
	if f.Default != nil &&
		// if the default value is not nil, the value is nil or emtpy and the type is NOT string,
		(((value == nil || *value == "") && f.Type != String) ||
			// or the value is nil and the type is string
			(value == nil && f.Type == String)) {
		selectedValue = f.Default
		// recursive call to validate default value
		// to avoid schema development errors

		// the default value is validated recursively
		if _, err := f.Validate(selectedValue); err != nil {
			return nil, errors.New("schema validation error on default value: " + err.Error())
		}
	} else {
		selectedValue = value

	}

	if selectedValue != nil {
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
				return nil, errors.New("invalid schema: `allowed_values` is required for enum type")
			}
			if IsEnum(*selectedValue, *separator, *f.AllowedValues) == false {
				return nil, errors.New("`value`: '" + *selectedValue + "' is not in: '" + *f.AllowedValues + "' separated by: '" + *separator + "'")
			}
		case FileBase64:
			if IsFile(*selectedValue) {
				fileInfo, err := os.Stat(*selectedValue)
				if err != nil {
					return nil, err
				}
				if fileInfo.Size() > MaxFileSize {
					return nil, fmt.Errorf("`%s` exceeds %d bytes", *selectedValue, MaxFileSize)
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
			} else {
				return nil, errors.New("file `" + *selectedValue + "` is not a file or is not accessible")
			}
		case File:
			if IsFile(*selectedValue) {
				if f.MountPath == nil {
					return nil, errors.New("invalid schema: `mountPath` is required for `file` type")
				}
			} else {
				return nil, errors.New("file `" + *selectedValue + "is not a file or is not accessible")
			}
		default:
			return nil, errors.New("impossible to validate object")
		}
	}
	return selectedValue, nil
}
