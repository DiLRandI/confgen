package appconfig_test

import (
	"fmt"
	"strings"

	"go-config/config"
	appconfig "go-config/examples"
)

func ExampleLoad() {
	cfg, err := appconfig.Load(
		config.Reader("embedded", strings.NewReader("server: {port: 9000}"), config.FormatYAML),
		config.Env(config.WithLookupEnv(func(name string) (string, bool) {
			values := map[string]string{"DATABASE_URL": "postgres://localhost/shop", "SHOP_SERVER_PORT": "10000"}
			v, ok := values[name]
			return v, ok
		})),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(cfg.Server.Host, cfg.Server.Port, cfg.Server.Timeout)
	// Output: 0.0.0.0 10000 30s
}

func ExampleMustLoad() {
	cfg := appconfig.MustLoad(config.Reader("inline", strings.NewReader(`{"database":{"url":"postgres://localhost/shop"},"debug":false}`), config.FormatJSON))
	fmt.Println(cfg.Server.Port, cfg.Debug)
	// Output: 8080 false
}
