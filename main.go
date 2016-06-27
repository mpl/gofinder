// 2011 - Mathieu Lonjaret

// The gofinder program is an acme user interface to look for include files,
// specific patterns, and regexps.
package main

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"

	"9fans.net/go/acme"
)

const (
	all      = "all"
	NBUF     = 512
	location = "loc"
)

const (
	regex = iota
	file
	fortFunc
	fortMod
	fortSub
	fortType
	cppInc
	cppClassMeth
	cppClassMemb
	goPack
	goFunc
	goMeth
	goTyp
	pyFunc
	doGetProjects
)

var (
	port = flag.String("p", "2020", "listening port")
	// TODO(mpl): handle other langs. or at least say we don't support them.
	// TODO(mpl): make it clear with usage which flags are for CLI use.
	flagProject = flag.String("project", "", "Name of the project to search in. Defaults to the first one found in the config otherwise.")
	flagWhere = flag.Int("where", 0, "Where to search for in the project. Defaults to first location found in the project otherwise.")
	flagFunc = flag.String("func", "", "The function to search for.")
	flagMethod = flag.String("method", "", "The method to search for.")
	flagPkg = flag.String("pkg", "", "The package to search for.")
	flagType = flag.String("type", "", "The type to search for.")
	help = flag.Bool("h", false, "show this help")
)

var (
	w              *acme.Win
	PLAN9          = os.Getenv("PLAN9")
	configFile     string
	lineBuf        []byte
	syntaxElements map[string][]string
	allExts        map[string][]string
	projectWord    = regexp.MustCompile(`^[a-zA-Z]+:`)
	resZone        string

	// Actually guards the whole of findRegex. I wanted to use it as well to
	// make killFind atomic, but don't think that's whe way to do it.
	findProcMu sync.Mutex

	killGrepMu sync.Mutex
	killGrep   bool
)

func initWindow() {
	var err error = nil
	w, err = acme.New()
	if err != nil {
		log.Fatal(err)
	}
	title := "gofind-" + configFile
	w.Name(title)
	tag := "Reload Kill"
	w.Write("tag", []byte(tag))
	err = reloadConf(configFile)
	if err != nil {
		log.Fatal(err)
	}
	lineBuf = make([]byte, NBUF)
}

func printUi() error {
	err := w.Addr("%s", "#0,")
	if err != nil {
		return err
	}
	w.Write("data", []byte(""))
	w.Write("body", []byte("Search in: \n"))
	w.Write("body", []byte("-----------------------------------"))
	w.Write("body", []byte("\n"))
	for _, v := range projects {
		w.Write("body", []byte(v.Name+":"))
		w.Write("body", []byte("\n"))
		for _, l := range v.Languages {
			w.Write("body", []byte("	"+l+":"))
			for _, el := range syntaxElements[l] {
				w.Write("body", []byte("	"+el))
			}
			w.Write("body", []byte("	"+all))
			w.Write("body", []byte("\n"))
		}
		for _, l := range v.Locations {
			w.Write("body", []byte("	"+l))
		}
		w.Write("body", []byte("\n"))
		for _, ex := range v.Excluded {
			w.Write("body", []byte("	"+ex))
		}
		w.Write("body", []byte("\n"))
	}
	w.Write("body", []byte("-----------------------------------"))
	w.Write("body", []byte("\n"))
	w.Write("body", []byte("\n"))
	w.Write("body", []byte("\n"))
	w.Write("body", []byte("History: \n"))
	w.Write("body", []byte("-----------------------------------\n"))
	// silly trick: select all the things to know the addr of eof
	err = w.Addr("%s", ",")
	if err != nil {
		return err
	}
	_, q1, err := w.ReadAddr()
	resZone = "#" + fmt.Sprint(q1)
	return nil
}

func reloadConf(configFile string) error {
	err := loadProjects(configFile)
	if err != nil {
		return err
	}
	err = printUi()
	if err != nil {
		return err
	}
	return nil
}

func sendCommand(code int, what string, where string) {
	c, err := net.Dial("tcp", "localhost:"+*port)
	if err != nil {
		log.Fatal(err)
	}
	enc := gob.NewEncoder(c)
	err = enc.Encode(msg{
		Action: code,
		What:   what,
		Where:  where,
	})
	if err != nil {
		log.Fatal("encode error:", err)
	}
	defer c.Close()
	if code != doGetProjects {
		return
	}
	
	if err := json.NewDecoder(c).Decode(&projects); err != nil {
		log.Fatalf("decode error: %v", err)
	}
}

type project struct {
	Name      string
	Languages []string
	Locations []string
	Exts      []string
	Excluded  []string
}

func loadProjects(file string) error {
	var loaded []project
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()
	dec := json.NewDecoder(r)
	err = dec.Decode(&loaded)
	if err != nil {
		return err
	}
	projects = make(map[string]project, 1)
	for _, v := range loaded {
		projects[v.Name] = v
	}
	return nil
}

func escapeSpecials(s string) string {
	escaped := strings.Replace(s, `(`, `\(`, -1)
	escaped = strings.Replace(escaped, `)`, `\)`, -1)
	escaped = strings.Replace(escaped, `*`, `\*`, -1)
	escaped = strings.Replace(escaped, `+`, `\+`, -1)
	escaped = strings.Replace(escaped, `?`, `\?`, -1)
	escaped = strings.Replace(escaped, `.`, `\.`, -1)
	return escaped
}

func dispatchSearch(from string, where string, what string) {
	//println(from)
	//println(where)
	//println(what)
	whereSplit := strings.Split(where, ":")
	proj := whereSplit[0]
	lang := whereSplit[1]
	v, ok := projects[proj]
	if !ok {
		log.Printf("%s not a valid project (not a key) \n", proj)
		return
	}
	// sanity checks
	if what == "" {
		return
	}
	found := false
	switch lang {
	case all:
		// search everywhere in the project
		found = true
	case location:
		// search only in a specific location (path)
		loc := whereSplit[2]
		for _, l := range v.Locations {
			if l == loc {
				found = true
				break
			}
		}
		if found == false {
			log.Printf("%s is not a location of %s project\n", loc, proj)
			return
		}
	default:
		// search only in one specific language (using the files extensions)
		for _, l := range v.Languages {
			if l == lang {
				found = true
				break
			}
		}
		if found == false {
			log.Printf("%s is not a language of %s project\n", lang, proj)
			return
		}
	}
	element := whereSplit[2]
	//TODO: rejoin the rest of where in case some ":" are present
	// TODO: this big switch is terrible. make a map instead.
	switch lang {
	case python:
		switch element {
		case pyFunction:
			sendCommand(pyFunc, what, proj+":"+lang)
		case all:
			sendCommand(regex, escapeSpecials(what), proj+":"+lang)
		}
	case golang:
		switch element {
		case goFunction:
			sendCommand(goFunc, what, proj+":"+lang)
		case goMethod:
			sendCommand(goMeth, what, proj+":"+lang)
		case goPackage:
			sendCommand(goPack, what, proj+":"+lang)
		case goType:
			sendCommand(goTyp, what, proj+":"+lang)
		case all:
			println("WHERE = " + proj+":"+lang)
			sendCommand(regex, escapeSpecials(what), proj+":"+lang)
		}
	case fortran:
		switch element {
		case fortranFunction:
			sendCommand(fortFunc, what, proj+":"+lang)
		case fortranModule:
			sendCommand(fortMod, what, proj+":"+lang)
		case fortranType:
			sendCommand(fortType, what, proj+":"+lang)
		case fortranSubroutine:
			sendCommand(fortSub, what, proj+":"+lang)
		case all:
			sendCommand(regex, escapeSpecials(what), proj+":"+lang)
		}
	case cpp:
		switch element {
		case cppInclude:
			sendCommand(cppInc, what, proj+":"+lang)
		case cppClassMethod:
			sendCommand(cppClassMeth, what, proj+":"+lang)
		case cppClassMember:
			sendCommand(cppClassMemb, what, proj+":"+lang)
		case all:
			sendCommand(regex, escapeSpecials(what), proj+":"+lang)
		}
	case all:
		println("WHERE = " + proj+":"+all)
		sendCommand(regex, what, proj+":"+all)
	default:
		// it's a path/location
		loc := lang
		println("WHERE = " + proj+":"+loc+":"+element)
		sendCommand(regex, escapeSpecials(what), proj+":"+loc+":"+element)
	}
}

func readDestination(e acme.Event) (string, error) {
	// read current line
	addr := "#" + fmt.Sprint(e.OrigQ0) + "+--"
	err := w.Addr("%s", addr)
	if err != nil {
		return "", err
	}
	_, err = w.Read("xdata", lineBuf)
	if err != nil {
		return "", err
	}
	if lineBuf[0] != '	' {
		proj := strings.Split(string(lineBuf), ":")[0]
		if !projectWord.MatchString(proj + ":") {
			return "", errors.New("wrong clic")
		}
		return proj + ":" + all, nil
	}
	language := location
	if strings.Contains(string(lineBuf), ":") {
		language = strings.TrimSpace(strings.Split(string(lineBuf), ":")[0])
	}
	elevator := ""
	for {
		// read line above
		elevator += "-"
		addr = "#" + fmt.Sprint(e.OrigQ0) + elevator + "+"
		err := w.Addr("%s", addr)
		if err != nil {
			return "", err
		}
		_, err = w.Read("xdata", lineBuf)
		if err != nil {
			return "", err
		}
		if lineBuf[0] != '	' {
			// found the project line
			proj := strings.Split(string(lineBuf), ":")[0]
			return proj + ":" + language, nil
		}
	}
	return "", nil
}

func eventLoop(c chan int) {
	for e := range w.EventChan() {
		switch e.C2 {
		case 'x': // execute in tag
			switch string(e.Text) {
			case "Del":
				w.Ctl("delete")
			case "Reload":
				err := reloadConf(configFile)
				if err != nil {
					log.Print(err)
				}
			case "Kill":
				killGrepMu.Lock()
				killGrep = true
				killGrepMu.Unlock()
			default:
				w.WriteEvent(e)
			}
		case 'X': // execute in body
			dest, err := readDestination(*e)
			if err != nil {
				log.Print(err)
				continue
			}
			//TODO: use another separator as ":" could be present in the chorded text
			where := dest + ":" + string(e.Text)
			dispatchSearch(string(e.Loc), where, string(e.Arg))
		case 'l': // button 3 in tag
			// let the plumber deal with it
			w.WriteEvent(e)
		case 'L': // button 3 in body
			// let the plumber deal with it
			w.WriteEvent(e)
		}
	}
	c <- 1
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: gofind projects.json \n")
	flag.PrintDefaults()
	os.Exit(2)
}

func loadSyntax() {
	syntaxElements = make(map[string][]string, 1)
	syntaxElements[golang] = goElements
	syntaxElements[python] = pyElements
	syntaxElements[fortran] = fortranElements
	syntaxElements[cpp] = cppElements
	allExts = make(map[string][]string, 1)
	allExts[golang] = goExts
	allExts[python] = pyExts
	allExts[fortran] = fortranExts
	allExts[cpp] = cppExts
}

func anyCmdFlag() bool {
	return *flagPkg != "" ||
		*flagFunc != "" ||
		*flagType != "" ||
		*flagMethod != ""
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
	}
	args := flag.Args()

	if flag.NArg() == 0 && !anyCmdFlag() {
		// TODO(mpl): update usage
		usage()
	}

	cmdMode := anyCmdFlag()
	if !cmdMode {
	if _, err := os.Stat(args[0]); err != nil {
		if !os.IsNotExist(err) {
			log.Fatal(err)
		}
		// 1st arg is not a config file, hence assume we want to send a command
		cmdMode = true
	}
	}

	if cmdMode {
		getProjects()
		sendCmd(args)
		return
	}

	configFile = flag.Args()[0]
	loadSyntax()
	initWindow()
	c := make(chan int)
	//TODO: window should not start if can't listen
	go listen(c)
	go eventLoop(c)
	<-c
	w.Ctl("delete")
	w.CloseFiles()
	// with an acme ui it's actually not necessary anymore  to have
	// a listening server, however I'm keeping it that way because:
	// 1) it's probably not big of a slowdown to send the requests to a server
	// wrt to the searches themselves
	// 2) it makes for a nice example of using gobs

}

func getProjects() {
	sendCommand(doGetProjects, "", "")
}

func sendCmd(args []string) {
		// TODO(mpl): make sure there's already an instance running in acme? assume for now.
	if len(projects) == 0 {
		println("no projects")
		return
	}
	proj := *flagProject
	var theProj project
	for k,v := range projects {
		if proj == "" {
			proj = k
			theProj = v
		}
		if proj == k {
			theProj = v
			break
		}
	}

	// TODO(mpl): check theProj valid at this point.

	theLoc := ""
	for i, l := range theProj.Locations {
		if i == *flagWhere {
			theLoc = l
		}
	}

	// where == proj:lang
	// lang == all|location|reallang

	// TODO(mpl): other langs? meh.
	code := regex
	var flagArg string
	switch {
	case *flagFunc != "":
		code = goFunc
		flagArg = *flagFunc
	case *flagMethod != "":
		code = goMeth
		flagArg = *flagMethod
	case *flagPkg != "":
		code = goPack
		flagArg = *flagPkg
	case *flagType != "":
		code = goTyp
		flagArg = *flagType
	}
	if code == regex {
		// TODO(mpl): probably wrong in some cases.
		what := escapeSpecials(strings.Join(args, " "))
		if theLoc == "" {
			sendCommand(code, what, proj+":go")
			return
		}
		sendCommand(code, what, proj+":loc:"+theLoc)
		return
	}
	sendCommand(code, flagArg, proj+":"+"go")
}

// click on "all": WHERE = camlistore:go
// click on "/home/mpl/src/camlistore.org": WHERE = camlistore:loc:/home/mpl/src/camlistore.org
