package schema_test

import (
	"fmt"
	"github.com/DiLRandI/confgen/schema"
	"testing"
)

func ExampleCompile() {
	m, err := schema.Compile("config.schema.yaml", []byte("version: 1\npackage: appconfig\nenv_prefix: APP\nfields:\n  api_url: {type: string, required: true}\n"))
	if err != nil {
		panic(err)
	}
	f := m.Descriptor.Fields[0]
	fmt.Println(f.GoName, f.EnvName)
	// Output: APIURL APP_API_URL
}

func BenchmarkCompile(b *testing.B) {
	data := []byte("version: 1\npackage: appconfig\nenv_prefix: APP\nfields:\n  host: {type: string, default: localhost}\n  port: {type: int, default: 8080, min: 1, max: 65535}\n  timeout: {type: duration, default: 30s}\n")
	b.ReportAllocs()
	for b.Loop() {
		if _, e := schema.Compile("bench", data); e != nil {
			b.Fatal(e)
		}
	}
}
