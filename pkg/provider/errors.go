package provider

import "errors"

// ErrLabelNotFound is returned when a required container image label is missing.
var ErrLabelNotFound = errors.New("label not found")

// ErrNotScenario is returned when an image explicitly declares that it is not a scenario.
var ErrNotScenario = errors.New("image is not a scenario")
