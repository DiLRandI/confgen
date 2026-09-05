package infer_test

import (
	"fmt"

	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
)

func ExampleFromConfig() {
	m, err := infer.FromConfig("config.yaml", []byte("port: 8080\ntimeout: 30s\n"), infer.Options{Package: "appconfig", CopyDefaults: true})
	if err != nil {
		panic(err)
	}
	b, err := schema.Render(m)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(b))
	// Output:
	// version: 1
	// package: appconfig
	// fields:
	//   port:
	//     type: int64
	//     default: 8080
	//   timeout:
	//     type: string
	//     default: 30s
}
