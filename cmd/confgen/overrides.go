package main

import (
	"github.com/DiLRandI/confgen/infer"
	inputlimit "github.com/DiLRandI/confgen/input"
)

func readOverrides(path string) (map[string]infer.TypeOverride, error) {
	if path == "" {
		return nil, nil
	}
	b, err := inputlimit.DefaultLimit.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return infer.ParseOverrides(path, b)
}
