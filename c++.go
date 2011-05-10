package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

const cpp = "c++"
const (
	member = iota
	staticMethod
	method
)

var (
	cppMethodValidator = regexp.MustCompile(`^.*\..*\);?$`)
	cppClassMethodValidator = regexp.MustCompile(`^.*::.*\($`)
	cppMemberValidator = regexp.MustCompile(`^.*::.*;?$`)
)

//TODO: merge this one and the one below
func findCppClassMethod(input string) {
	v, ok := projects["casa"]
	if !ok {
		log.Printf("%s not a key in projects \n", "casa")
		return
	}
	// first parse class name and find the file
	// assume class is in file with same name
//TODO: do not assume what's above
	fields := strings.Split(input, "::", -1)
	className := strings.Split(fields[0], "<", -1)[0]
	method := fields[1][0:len(fields[1])-1]
	methodValidator := regexp.MustCompile(className + ".*::" + method)
	println(className)
//TODO: do not hardcode the extension
	fullPath := findFile(className + ".cc", v.Locations)
	println(fullPath)
	// then find exact location in the file
	f, err := os.Open(fullPath)
	if err != nil {
		log.Printf("%v \n", err)
		return
	}
	bufr := bufio.NewReader(f)
	i := 1
	for {
		line, err := bufr.ReadString('\n')
		if err != nil {
			log.Printf("%v \n", err)
			f.Close()
			return
		}
//TODO: template and args matching ?
		if methodValidator.MatchString(line) {
			break
		}
		i++
	}
	f.Close()
	fmt.Printf("%s:%d \n", fullPath, i)
}

//TODO: merge this one and the one above
func findCppMethod(input string) {
	v, ok := projects["casa"]
	if !ok {
		log.Printf("%s not a key in projects \n", "casa")
		return
	}
	var pattern *regexp.Regexp
	fullPath := ""
	altSearch := ""
	// first parse class name and find the file
	// assume class is in file with same name
	if !strings.Contains(input, "(") {
		// look for a class member
		fields := strings.Split(input, "::", -1)
//TODO: probably needs to filter "static", "const", etc...
		className := strings.TrimSpace(strings.Split(fields[0], "<", -1)[0])
		classMember := strings.Split(fields[1], ";", -1)[0]
		pattern = regexp.MustCompile(".* +" + classMember + " *;")
		fullPath = findFile(className + ".h", v.Locations)
		altSearch = classMember
	} else {
		firstCut := strings.Split(input, ".", -1)
		l := len(firstCut)
		if l < 2 {
			log.Printf("pb when Fielding \n")
			return
		}
		// do not count calls in parentheses 
		pieces := []string{firstCut[0], firstCut[1]}
		parenCount := 0
		for i:=2; i<l; i++ {
			parenCount += strings.Count(firstCut[i-1], "(")
			parenCount -= strings.Count(firstCut[i-1], ")")
			if parenCount == 0 {
				pieces = append(pieces, firstCut[i])
			}
		}
		l = len(pieces)
		if l < 2 {
			log.Printf("pb when Fielding \n")
			return
		}
		finalMethod := strings.Split(pieces[l-1], "(", -1)[0]
		className := strings.TrimSpace(pieces[0])
		line := ""
		for i:=1; i < l-1; i++ {
			method := strings.Split(pieces[i], "(", -1)[0]
//TODO: do not hardcode the extension
			fullPath = findFile(className + ".h", v.Locations)
			// then find exact location in the file
			f, err := os.Open(fullPath)
			if err != nil {
				log.Printf("%v \n", err)
				return
			}
			bufr := bufio.NewReader(f)
			for {
				line, err = bufr.ReadString('\n')
				if err != nil {
					f.Close()
					if err == os.EOF {
						goto happyEnding
					}
					log.Printf("%v \n", err)
					return
				}
//TODO: template and args matching ?
				if strings.Contains(line, " " + method) {
					break
				}
			}
			f.Close()
			fields := strings.Fields(line)
			if len(fields) < 2 {
				log.Printf("pb when Fielding \n")
				return
			}
//TODO: better method: take the one that is just before the method name
			for i:= 0; i< len(fields); i++ {
				if fields[i] != "const" || fields[i] != "static" {
					className = fields[i]
					break
				}
			}
			//now strip "&" or "*"
			le := len(className) - 1
			if className[le] == '&' || className[le] == '*'	{
				className = className[0:le]
			}
		}
//TODO: replace it with something that returns but doesn't fuck up the server when failing
		pattern = regexp.MustCompile(className + ".*::" + finalMethod)
		fullPath = findFile(className + ".cc", v.Locations)
		altSearch = ".*::" + finalMethod
	}
	// then find exact location in the file
	f, err := os.Open(fullPath)
	if err != nil {
		log.Printf("%v \n", err)
		return
	}
	ln := 1
	line := ""
	bufr := bufio.NewReader(f)
	for {
		line, err = bufr.ReadString('\n')
		if err != nil {
			f.Close()
			if err == os.EOF {
				goto happyEnding
			}
			log.Printf("%v \n", err)
			return
		}
//TODO: template and args matching ?
		if pattern.MatchString(line) {
			break
		}
		ln++
	}
	f.Close()
	fmt.Printf("%s:%d \n", fullPath, ln)
	return
happyEnding:
	findRegex(altSearch, v.Locations, v.Exts)
}
