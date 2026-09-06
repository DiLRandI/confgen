package main

import (
	"os"

	"github.com/DiLRandI/confgen/infer"
)

func readOverrides(path string) (map[string]infer.TypeOverride, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return infer.ParseOverrides(path, b)
}
