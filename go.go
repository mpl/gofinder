package main

import (
//	"strings"
)

const (
	golang = "go"
	goType = "type"
	goFunction = "func"
)

var (
//	goPackageValidator = regexp.MustCompile(`^( | 	)?"[a-zA-Z]+(/[a-zA-Z]+)*"`)
	goElements = []string{goFunction}
	goExts = []string{`\.go`}
)

//TODO: when target is in name, find the right one. (hard. need from).
func findGoFunc(name string, where []string) {
	regex := `^` + goFunction + ".*" + name
	findRegex(regex, where, goExts)
}
