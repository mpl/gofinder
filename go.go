package main

import (
	"regexp"
)

const golang = "go"

var (
	goPackageValidator = regexp.MustCompile(`^( | 	)?"[a-zA-Z]+(/[a-zA-Z]+)*"`)
)

