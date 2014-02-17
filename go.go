package main

import (
	"strings"
)

const (
	golang     = "go"
	goType     = "type"
	goFunction = "func"
	goMethod   = "method"
	goPackage  = "package"
)

var (
	//	goPackageValidator = regexp.MustCompile(`^( | 	)?"[a-zA-Z]+(/[a-zA-Z]+)*"`)
	goElements = []string{goPackage, goFunction, goType, goMethod}
	goExts     = []string{`\.go`}
)

func cleanGoPackageLine(input string) string {
	return strings.TrimSpace(strings.Replace(input, `"`, "", -1))
}

func findGoFunc(name string, where []string, excl []string) {
	regex := `^` + goFunction + " +" + name + ` *\(`
	findRegex(regex, where, goExts, excl)
}

//TODO: when target is in name, find the right one. (hard. need from).
func findGoMeth(name string, where []string, excl []string) {
	regex := `^` + goFunction + ` +\(.*\) +` + name + ` *\(`
	findRegex(regex, where, goExts, excl)
}

func findGoType(name string, where []string, excl []string) {
	regex := `^` + goType + " +" + name
	findRegex(regex, where, goExts, excl)
}
