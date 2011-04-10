package main

import (
	"regexp"
	"strings"
)

const fortranExt = `\.f90`

var (
	fortranCallValidator = regexp.MustCompile(".*call.*")
	fortranUseModuleValidator = regexp.MustCompile(".*use.*")
)

func findFortranSubroutine(call string) {
//TODO: match the number of args of the subroutine
	findRegex("^subroutine " + strings.Split(call, "(", -1)[0] + `(.*`,
		fortranIncludes, fortranExt)
}

func findFortranModule(module string) {
	findRegex("^module " + module,
		fortranIncludes, fortranExt)
}

