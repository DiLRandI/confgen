// Package appconfig demonstrates generated configuration and executable examples.
// Regenerate its Go types and templates with go generate ./... from examples.
package appconfig

//go:generate go run github.com/DiLRandI/confgen/cmd/configgen -schema config.schema.yaml -out config_gen.go -example-yaml ../config.example.yaml -example-env ../generated.env.example
