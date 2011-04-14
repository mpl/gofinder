package main

import (
	"log"
	"regexp"
	"strings"
)

const fortran = "fortran"

var (
	fortranCallValidator = regexp.MustCompile(".*call.*")
	fortranUseModuleValidator = regexp.MustCompile(".*use.*")
)

func findFortranSubroutine(call string) {
	v, ok := langs[fortran]
	if !ok {
		log.Printf("%s not a key in langs \n", fortran)
		return
	}
//TODO: match the number of args of the subroutine
	findRegex("^subroutine " + strings.Split(call, "(", -1)[0] + `(.*`,
		v.Locations, v.Ext)
}

func findFortranModule(module string) {
	v, ok := langs[fortran]
	if !ok {
		log.Printf("%s not a key in langs \n", fortran)
		return
	}
	findRegex("^module " + module,
		v.Locations, v.Ext)
}

