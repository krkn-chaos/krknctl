package typing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/krkn-chaos/krknctl/pkg/utils"
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
	Secret           bool    `json:"secret,omitempty"`
}

type alias InputField

func (f *InputField) UnmarshalJSON(data []byte) error {
	aux := &struct {
		*alias
		Name     *string `json:"name"`
		Type     *string `json:"type"`
		Variable *string `json:"variable"`
		Required *string `json:"required"`
		Secret   *string `json:"secret"`
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
	// the envvar to be exported in the scenario_orchestrator
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

	if fieldProperty, ok := temp["secret"]; ok {
		secret, err := strconv.ParseBool(fieldProperty)
		if err != nil {
			return err
		}
		f.Secret = secret
	} else {
		f.Secret = false
	}

	return nil
}

func (f *InputField) Validate(value *string) (*string, error) {
	var deferErr error
	// if string value is nil, the default value is nil and the field is required the field is not valid
	if value == nil && f.Default == nil && f.Required && f.Type == String {
		return nil, errors.New("`" + f.Type.String() + " can be blank, but not null without a default if required")
	}
	// if any other type value is nil or empty, the default value is nil or empty and the field is required the field is not valid
	if (value == nil || *value == "") && (f.Default == nil || *f.Default == "") && f.Required && f.Type != String {
		return nil, fmt.Errorf("field `%s` doesn't have a `default` and cannot be nil or empty for type `%s`", *f.Name, f.Type)
	}

	var selectedValue *string
	// if the default value is not nil
	if f.Default != nil && // if the default value is not nil
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
				if !match {
					return nil, errors.New("`value`: '" + *selectedValue + "' does not match `validator`: '" + *f.Validator + "'")
				}
			}
		case Number:
			if !IsNumber(*selectedValue) {
				return nil, errors.New("`value`: '" + *selectedValue + "' is not a number")
			}
		case Boolean:
			if !IsBoolean(*selectedValue) {
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
			if !IsEnum(*selectedValue, *separator, *f.AllowedValues) {
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

				defer func() {
					deferErr = file.Close()
				}()

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
			var err error
			selectedValue, err = utils.ExpandFolder(*selectedValue, nil)
			if err != nil {
				return nil, err
			}
			if IsFile(*selectedValue) {
				if f.MountPath == nil || *f.MountPath == "" {
					return nil, errors.New("mount path not set in schema")
				}
			} else {
				return nil, errors.New("file `" + *selectedValue + "` is not a file or is not accessible")
			}
		default:
			return nil, errors.New("impossible to validate object")
		}
	}
	return selectedValue, deferErr
}
