package main

import (
	"strings"
)

const (
	golang = "go"
	goType = "type"
	goFunction = "func"
	goPackage = "package"
)

var (
//	goPackageValidator = regexp.MustCompile(`^( | 	)?"[a-zA-Z]+(/[a-zA-Z]+)*"`)
	goElements = []string{goPackage, goFunction, goType}
	goExts = []string{`\.go`}
)

func cleanGoPackageLine(input string) string {
	return strings.TrimSpace(strings.Replace(input, `"`, "", -1))
}

//TODO: when target is in name, find the right one. (hard. need from).
func findGoFunc(name string, where []string) {
	regex := `^` + goFunction + ".*" + name
	findRegex(regex, where, goExts)
}

func findGoType(name string, where []string) {
	regex := `^` + goType + " +" + name
	findRegex(regex, where, goExts)
}
