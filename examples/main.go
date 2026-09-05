// Command examples demonstrates a consumer of confgen in a separate Go module.
package main

import (
	"fmt"
	"log"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/examples/appconfig"
)

func main() {
	cfg, err := appconfig.Load(config.OptionalFile("config.yaml"), config.Env())
	if err != nil {
		log.Fatal(err)
	}
	// Select non-secret fields explicitly. Do not log the entire config struct.
	fmt.Printf("server=%s:%d timeout=%s debug=%t\n", cfg.Server.Host, cfg.Server.Port, cfg.Server.Timeout, cfg.Debug)
}
