// Package config provides sources for generated configuration packages.
//
//	cfg, err := appconfig.Load(
//	    config.OptionalFile("config.yaml"),
//	    config.Env(),
//	)
//
// Later sources override earlier ones. Schema defaults have the lowest priority.
// File reads YAML or JSON; Reader loads an io.Reader; Env reads declared variables.
// Use errors.As to inspect *Error when loading fails.
//
// Applications normally use generated Load, LoadContext, and MustLoad helpers.
// Descriptor, Document, and generic Load support advanced integrations.
package config
