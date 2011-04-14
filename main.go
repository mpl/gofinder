package main

import (
	"flag"
	"fmt"
	"gob"
//	"io/ioutil"
	"json"
	"log"
	"net"
	"os"
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
	addLoc
	listLocs
	setLang
)

//TODO: make a server of it. takes message to add/set includes, languages, etc...
//TODO: use json as a config files for all the languages, so that user can add them without touching source.
var (
	daemon = flag.Bool("d", false, "starts the gofind server")
	reg = flag.String("r", "", "regexp to search for")
	langsFile = flag.String("l", "", "json file with languages infos")
	port = flag.String("p", "2020", "listening port")
	help = flag.Bool("h", false, "show this help")
)

func sendCommand(code int, what string, where string) {
	c, err := net.Dial("tcp", "", "localhost:" + *port)
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

type language struct {
	Name string
	Locations []string
	Ext string
}

func loadLanguages(file string) {
	var loaded []language
	r, err := os.Open(file, os.O_RDONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(r)
	err = dec.Decode(&loaded)
	if err != nil {
		log.Fatal(err)
	}	
	for _,v := range loaded {
		langs[v.Name] = v
	}
	//update allCode
	allLangs := language{Name:all}
	for _,v := range langs {
		allLangs.Ext = allLangs.Ext + v.Ext + "|"
		for _,l := range v.Locations {
			allLangs.Locations = append(allLangs.Locations, l)
		}
	}
	allLangs.Ext = allLangs.Ext[0:len(allLangs.Ext)-1]
	langs[allLangs.Name] = allLangs
	r.Close()
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: \n\t gofind -d \n");
	fmt.Fprintf(os.Stderr, "\t gofind file_path \n");
	fmt.Fprintf(os.Stderr, "\t gofind -r regexp \n");
	fmt.Fprintf(os.Stderr, "\t gofind cmd \n");
//TODO: explain cmd
	flag.PrintDefaults();
	os.Exit(2);
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
	}

	if flag.NArg() == 0 {
		if *daemon {
			if *langsFile != "" {
				loadLanguages(*langsFile)
			}
			listen()
			return
		}
		if *reg == "" {
			usage()
		}
		println("regexp: "+*reg)
		sendCommand(regex, *reg, "")
		return
	}

	arg0 := flag.Args()[0]
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
//TODO: usage if > 3
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
	case fortranCallValidator.MatchString(arg0):
		sendCommand(fortranSubroutine, arg1, arg1)
	case fortranUseModuleValidator.MatchString(arg0):
		sendCommand(fortranModule, arg1, arg1)
	case arg0 == addKw:
		if flag.NArg() < 3 {
			usage()
		}
		sendCommand(addLoc, arg2, arg1)
	case arg0 == listKw:
		sendCommand(listLocs, "", arg1)
	case arg0 == setKw:
		sendCommand(setLang, arg1, arg1)
	default:
		sendCommand(file, arg0, arg1)
	}
}
