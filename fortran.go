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

//TODO: factorize 
func findFortranSubroutine(call string) {
	if proj ==  "" {
		// if no specified proj, just go through all fortran projects
		for _,v := range projects {
			if v.Language == fortran {
				//TODO: match the number of args of the subroutine
				subroutine := ""
				if strings.Contains(call, "(") {
					subroutine = strings.TrimSpace(strings.Split(call, "(", -1)[0])
				} else {
					subroutine = strings.TrimSpace(call)
				}
				findRegex("^subroutine " + subroutine + ` *(.*`,
					v.Locations, v.Exts)
			}
		}
	} else {
		v, ok := projects[proj]
		if !ok {
			log.Printf("%s not a key in projects \n", proj)
			return
		}
		//TODO: match the number of args of the subroutine
		subroutine := ""
		if strings.Contains(call, "(") {
			subroutine = strings.TrimSpace(strings.Split(call, "(", -1)[0])
		} else {
			subroutine = strings.TrimSpace(call)
		}
		findRegex("^subroutine " + subroutine + `(.*`,
			v.Locations, v.Exts)
	}
}

//TODO: factorize 
func findFortranModule(module string) {
	if proj ==  "" {
		// if no specified proj, just go through all fortran projects
		for _,v := range projects {
			if v.Language == fortran {
				//TODO: match the number of args of the subroutine
				findRegex("^module " + module,
					v.Locations, v.Exts)
			}
		}
	} else {
		v, ok := projects[proj]
		if !ok {
			log.Printf("%s not a key in projects \n", proj)
			return
		}
		//TODO: match the number of args of the subroutine
		findRegex("^module " + module,
			v.Locations, v.Exts)
	}
}

