package provider

import "errors"

// ErrLabelNotFound is returned when a required container image label is missing.
var ErrLabelNotFound = errors.New("label not found")
