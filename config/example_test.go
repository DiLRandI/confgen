package config_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DiLRandI/confgen/config"
)

func ExampleLoad() {
	// Most applications use generated wrappers. A custom descriptor can load
	// an existing struct without interpreting struct tags.
	type App struct{ Port int }
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "port", Path: "port", Kind: config.KindInt, GoIndex: []int{0}, Required: true}}}
	cfg, err := config.Load[App](context.Background(), d, config.Reader("inline", strings.NewReader("port: 0"), config.FormatYAML))
	if err != nil {
		panic(err)
	}
	fmt.Println(cfg.Port)
	// Output: 0
}

func ExampleError() {
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Path: "port", Kind: config.KindInt, GoIndex: []int{0}, Required: true}}}
	_, err := config.Load[struct{ Port int }](context.Background(), d)
	if configErr, ok := errors.AsType[*config.Error](err); ok {
		fmt.Println(configErr.Issues[0].Path, configErr.Issues[0].Kind)
	}
	// Output: port required
}
