/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"io"
	"os"
	"runtime"

	isatty "github.com/mattn/go-isatty"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
)

// isSmartTerminal returns true if w is a terminal with VT escape code support.
// Inlined from sigs.k8s.io/kind/pkg/internal/env to avoid internal package import.
func isSmartTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	if !isatty.IsTerminal(f.Fd()) {
		return false
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	term := os.Getenv("TERM")
	if term == "dumb" || term == "st-256color" {
		return false
	}
	if runtime.GOOS == "windows" && os.Getenv("WT_SESSION") == "" {
		return false
	}
	return true
}

// NewLogger returns the standard logger used by the kind CLI
// This logger writes to os.Stderr
func NewLogger() log.Logger {
	var writer io.Writer = os.Stderr
	if isSmartTerminal(writer) {
		writer = cli.NewSpinner(writer)
	}
	return cli.NewLogger(writer, 0)
}

// ColorEnabled returns true if color is enabled for the logger
// this should be used to control output
func ColorEnabled(logger log.Logger) bool {
	type maybeColorer interface {
		ColorEnabled() bool
	}
	v, ok := logger.(maybeColorer)
	return ok && v.ColorEnabled()
}
