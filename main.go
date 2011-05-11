package main

import (
	"flag"
	"fmt"
	"gob"
	"json"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
)

const addKw = "add"
const listKw = "list"
const setKw = "set"
const all = "all"
const (
	regex = iota
	file
	fortranSubroutine
	fortranModule
	cppClassMethod
	cppClassMember
	cppMethod
	addLoc
	listLocs
	setProj
)

var (
	daemon    = flag.Bool("d", false, "starts the gofind server")
	reg       = flag.String("r", "", "regexp to search for")
	port      = flag.String("p", "2020", "listening port")
	noplumb   = flag.Bool("noplumb", false, "do not use the plumber/acme")
	help      = flag.Bool("h", false, "show this help")
)

func sendCommand(code int, what string, where string) {
	c, err := net.Dial("tcp", "localhost:"+*port)
	if err != nil {
		log.Fatal(err)
	}
	enc := gob.NewEncoder(c)
	err = enc.Encode(msg{code, what, where})
	if err != nil {
		log.Fatal("encode error:", err)
	}
	c.Close()
}

type project struct {
	Name      string
	Language string
	Locations []string
	Exts      []string
}

func loadProjects(file string) {
	var loaded []project
	r, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(r)
	err = dec.Decode(&loaded)
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range loaded {
		projects[v.Name] = v
	}
	//update allCode
	allProjects := project{Name: all}
	for _, v := range projects {
		allProjects.Exts = append(allProjects.Exts, v.Exts...)
		for _, l := range v.Locations {
			allProjects.Locations = append(allProjects.Locations, l)
		}
	}
	projects[allProjects.Name] = allProjects
	filePathValidator = regexp.MustCompile(".*(" + strings.Join(allProjects.Exts, "|") + ")$")	
	r.Close()
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: \n\t gofind -d projects.json \n")
	fmt.Fprintf(os.Stderr, "\t gofind classname|filename|... \n")
	fmt.Fprintf(os.Stderr, "\t gofind -r regexp \n")
	fmt.Fprintf(os.Stderr, "\t gofind list [projectname] \n")
	fmt.Fprintf(os.Stderr, "\t gofind set [projectname] \n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
	}
	
	if flag.NArg() == 0 {
		if *reg == "" {
			usage()
		}
		println("regexp: " + *reg)
		sendCommand(regex, *reg, "")
		return
	}

	arg0 := flag.Args()[0]
	if *daemon {
		loadProjects(arg0)
		listen()
		return
	}

	arg1 := ""
	arg2 := ""
	chorded := strings.Fields(arg0)
	switch len(chorded) {
	case 1:
		// not from chording
		if flag.NArg() >= 2 {
			arg1 = flag.Args()[1]
			if flag.NArg() >= 3 {
				arg2 = flag.Args()[2]
			}
		}
	case 3:
		// from chording
		arg2 = chorded[2]
		fallthrough
	case 2:
		// from chording
		arg0 = chorded[0]
		arg1 = chorded[1]
	default:
		usage()
	}

	switch {
	case cppMethodValidator.MatchString(arg0) || cppMemberValidator.MatchString(arg0):
		sendCommand(cppMethod, arg0, arg1)	
	case cppClassMethodValidator.MatchString(arg0):
		sendCommand(cppClassMethod, arg0, arg1)	
	case fortranCallValidator.MatchString(arg0):
		sendCommand(fortranSubroutine, arg1, arg1)
	case fortranUseModuleValidator.MatchString(arg0):
		sendCommand(fortranModule, arg1, arg1)
//TODO: remove?
	case arg0 == addKw:
		if flag.NArg() < 3 {
			usage()
		}
		sendCommand(addLoc, arg2, arg1)
	case arg0 == listKw:
		sendCommand(listLocs, "", arg1)
	case arg0 == setKw:
		sendCommand(setProj, arg1, arg1)
	default:
		sendCommand(file, arg0, arg1)
	}
}
