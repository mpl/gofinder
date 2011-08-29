package main

import (
	"strings"
)

const (
	fortran = "fortran"
	fortranModule = "module"
	fortranSubroutine = "subroutine"
	fortranFunction = "function"
)

var (
	fortranElements = []string{fortranModule, fortranSubroutine, fortranFunction}
	fortranExts = []string{`\.f90`}
)

func findFortranSubroutine(call string, where []string) {
	//TODO: match the sig of the subroutine
	findRegex(`^` + fortranSubroutine + ` +` + strings.TrimSpace(call) + ` *\(.*`,
		where, fortranExts)
}

func findFortranModule(module string, where []string) {
	findRegex(`^` + fortranModule + ` +` + strings.TrimSpace(module),
		where, fortranExts)
}

func findFortranFunction(call string, where []string) {
	findRegex(`^` + fortranFunction + ` +` + strings.TrimSpace(call) +
	` *\(.*`, where, fortranExts)
}
