package main

const (
	python     = "python"
	pyFunction = "def"
	pyModule   = "module"
)

var (
	pyElements = []string{pyFunction, pyModule}
	pyExts     = []string{`\.py`}
)

func findPyFunc(name string, where []string) {
	regex := `^` + pyFunction + " +" + name + ` *\(`
	findRegex(regex, where, pyExts, nil)
}
