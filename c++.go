package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

//TODO: redo it all to work with the acme ui
const (
	cpp            = "c++"
	cppInclude     = "include"
	cppClassMethod = "staticMethod"
	cppClassMember = "staticMember"
)

var (
	cppElements = []string{cppClassMethod, cppClassMember, cppInclude}
	// note: keep .h as the last one
	cppExts = []string{`\.cc`, `\.cpp`, `\.h`}
)

func findCppClassMethod(call string, where []string) {
	// first parse class name and find the file
	// assume class is in file with same name
	//TODO: do not assume what's above
	fields := strings.Split(call, "::")
	className := strings.Split(fields[0], "<")[0]
	method := fields[1][0 : len(fields[1])-1]
	methodValidator := regexp.MustCompile(className + ".*::" + method)
	fullPath := ""
	for _, ext := range cppExts[0 : len(cppExts)-1] {
		fullPath = findFile(className+strings.Replace(ext, `\.`, `.`, 1), where)
		if fullPath != "" {
			break
		}
	}
	if fullPath == "" {
		log.Println("cpp class not found")
		return
	}
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

func findCppClassMember(name string, where []string) {
	// first parse class name and find the file
	// assume class is in file with same name
	//TODO: do not assume what's above
	fields := strings.Split(name, "::")
	className := strings.Split(fields[0], "<")[0]
	member := strings.TrimSpace(fields[1][0:len(fields[1])])
	memberValidator := regexp.MustCompile(".* +" + member + " *(;|,)")
	fullPath := findFile(className+strings.Replace(cppExts[len(cppExts)-1], `\.`, `.`, 1), where)
	if fullPath == "" {
		log.Printf("cpp class file header not found for %s \n", className)
		return
	}
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
		if memberValidator.MatchString(line) {
			break
		}
		i++
	}
	f.Close()
	fmt.Printf("%s:%d \n", fullPath, i)
}
