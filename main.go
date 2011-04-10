package main

import (
	"flag"
	"fmt"
	"gob"
	"log"
	"net"
	"os"
	"strings"
)

const addKw = "add"
const listKw = "list"
const setKw = "set"

const cExt = `\.h|\.c`
const cppExt = `\.h|\.cc`
const allExt = cExt + "|" + cppExt + "|" + fortranExt

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
	help = flag.Bool("h", false, "show this help")
)

func sendCommand(code int, what string, where int) {
	c, err := net.Dial("tcp", "", "localhost:2020")
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
			listen()
			return
		}
		if *reg == "" {
			usage()
		}
		println("regexp: "+*reg)
		sendCommand(regex, *reg, allZones)
		return
	}

	arg0 := flag.Args()[0]
	arg1 := ""
	arg2 := ""
	where := allZones
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

	switch arg1 {
	case "fortran":
		where = fortranZone
	case "cpp":
		where = cppZone
	default:
		where = allZones
	}
	switch {
	case fortranCallValidator.MatchString(arg0):
		sendCommand(fortranSubroutine, arg1, fortranZone)
	case fortranUseModuleValidator.MatchString(arg0):
		sendCommand(fortranModule, arg1, fortranZone)
	case arg0 == addKw:
		if flag.NArg() < 3 {
			usage()
		}
		sendCommand(addLoc, arg2, where)
	case arg0 == listKw:
		sendCommand(listLocs, "", where)
	case arg0 == setKw:
		sendCommand(setLang, arg1, where)
	default:
// Y U NO WORK?
		sendCommand(file, arg0, 9000)
	}
}
