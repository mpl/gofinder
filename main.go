// Copyright 2011 Mathieu Lonjaret

// The gofinder program is an acme user interface to search through Go projects.
// It uses 2-1 chording (see https://swtch.com/plan9port/man/man1/acme.html).
// It uses a JSON configuration file to define project(s) to search on; see
// projects-example.json for a working configuration example.
//
// It displays, in the following order: The name of the project, to perform a
// global search. The Go Guru (golang.org/x/tools/cmd/guru) modes, to perform a
// guru search. The project's locations, to perform a local search. For example,
// with the provided projects-example.json, the UI will look like:
//
//	Search in:
//	-----------------------------------
//	camlistore:
//		callees	callers	callstack	definition	describe	freevars	implements	peers	pointsto	referrers	what	whicherrs
//		/home/mpl/src/camlistore.org	/home/mpl/src/camlistore.org/vendor	/home/mpl/src/go4.org	/home/mpl/src/github.com/mpl
//	-----------------------------------
//
//
// The configuration file is mapped to a project type, which is defined as follows:
//
//	type Project struct {
//		// Name is the one word name describing the project, that will appear at
//		// the top of the UI. One word, because chording on the name starts a
//		// global search in the project. Global search means a find on all the
//		// files ending with the extensions defined in Exts, looking in the
//		// locations defined in Locations, excluding all the patterns defined in
//		// Excluded. The results are piped to a grep for the argument that is sent
//		// with the chord.
//		Name      string
//		// Locations defines all the locations relevant to the project, and as
//		// such, they are displayed on the UI. A global search runs find through
//		// all of them. A chording on one of the locations will perform a local
//		// search, i.e. in the same way as a global search, except find will only
//		// run through that one location.
//		Locations []string
//		// Exts defines the file extension patterns (regexp), that find will
//		// take into account. It defaults to []string{"\.go"} otherwise.
//		Exts      []string
//		// Excluded defines the patterns (regexp), that find will take into
//		// account to exclude from the search results.
//		Excluded  []string
//		// GuruScope is the scope that guru will use for the modes that need one.
//		GuruScope []string
//	}
package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	"9fans.net/go/acme"
)

const (
	guruKeyword     = "guru"
	locationKeyword = "location"
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
	help = flag.Bool("h", false, "show this help")
	// CLI mode flags. disabled for now, until feature is finished.
	// flagProject = flag.String("project", "", "Name of the project to search in. Defaults to the first one found in the config otherwise.")
	// flagWhere = flag.Int("where", 0, "Where to search for in the project. Defaults to first location found in the project otherwise.")
	// flagFunc = flag.String("func", "", "The function to search for.")
	// flagMethod = flag.String("method", "", "The method to search for.")
	// flagPkg = flag.String("pkg", "", "The package to search for.")
	// flagType = flag.String("type", "", "The type to search for.")
)

var (
	w           *acme.Win
	PLAN9       = os.Getenv("PLAN9")
	configFile  string
	projectWord = regexp.MustCompile(`^[a-zA-Z]+:`)
	resZone     string

	// Actually guards the whole of findRegex. I wanted to use it as well to
	// make killFind atomic, but don't think that's whe way to do it.
	findProcMu sync.Mutex

	killGrepMu sync.Mutex
	killGrep   bool

	// maps guru mode to whether it needs a scope
	guruModes = map[string]bool{
		"callees":    true,
		"callers":    true,
		"callstack":  true,
		"definition": false,
		"describe":   false,
		"freevars":   false,
		"implements": false,
		"peers":      true,
		"pointsto":   true,
		"referrers":  false,
		"what":       false,
		"whicherrs":  false,
	}
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
		var guruSorted []string
		for mode, _ := range guruModes {
			guruSorted = append(guruSorted, mode)
		}
		sort.Strings(guruSorted)
		for _, mode := range guruSorted {
			w.Write("body", []byte("	"+mode))
		}
		w.Write("body", []byte("\n"))
		for _, l := range v.Locations {
			w.Write("body", []byte("	"+l))
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

func sendCommand(code int, q *query) {
	c, err := net.Dial("tcp", "localhost:"+*port)
	if err != nil {
		log.Fatal(err)
	}
	enc := gob.NewEncoder(c)
	err = enc.Encode(msg{
		Action:  code,
		Project: q.project,
		What:    q.what,
		Where:   q.where,
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

type Project struct {
	// Name is the one word name describing the project, that will appear at
	// the top of the UI. One word, because chording on the name starts a
	// global search in the project. Global search means a find on all the
	// files ending with the extensions defined in Exts, looking in the
	// locations defined in Locations, excluding all the patterns defined in
	// Excluded. The results are piped to a grep for the argument that is sent
	// with the chord.
	Name string
	// Locations defines all the locations relevant to the project, and as
	// such, they are displayed on the UI. A global search runs find through
	// all of them. A chording on one of the locations will perform a local
	// search, i.e. in the same way as a global search, except find will only
	// run through that one location.
	Locations []string
	// Exts defines the file extension patterns (regexp), that find will
	// take into account. It defaults to []string{"\.go"} otherwise.
	Exts []string
	// Excluded defines the patterns (regexp), that find will take into
	// account to exclude from the search results.
	Excluded []string
	// GuruScope is the scope that guru will use for the modes that need one.
	GuruScope []string
}

func loadProjects(file string) error {
	var loaded []Project
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
	projects = make(map[string]Project, 1)
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

func dispatchSearch(q *query) {
	proj := q.project
	kind := q.kind
	where := q.where
	what := q.what
	v, ok := projects[proj]
	if !ok {
		log.Printf("%s not a valid project (not a key) \n", proj)
		return
	}
	// sanity checks
	if what == "" {
		return
	}

	if kind == guruKeyword {
		// TODO(mpl): move the guru call to the "server"? Not really a win,
		// but just out of consistency.
		if err := guru(q.mode, q.where, q.project); err != nil {
			log.Printf("go guru error: %v", err)
		}
		return
	}

	if kind != locationKeyword {
		log.Printf("unknown kind of search: %v", q.kind)
		return
	}
	if where != "" {
		found := false
		// search only in a specific location (path)
		for _, l := range v.Locations {
			if l == where {
				found = true
				break
			}
		}
		if found == false {
			log.Printf("%s is not a location of %s project\n", where, proj)
			return
		}
	}
	q.what = escapeSpecials(what)
	sendCommand(regex, q)
}

type query struct {
	project string
	kind    string // "location" for location search, or "guru", or "everywhere".
	mode    string // the guru mode if kind is "guru".
	where   string
	what    string
}

func buildQuery(e acme.Event) (*query, error) {
	const NBUF = 512
	line := make([]byte, NBUF)
	// read current line
	addr := "#" + fmt.Sprint(e.OrigQ0) + "+--"
	err := w.Addr("%s", addr)
	if err != nil {
		return nil, err
	}
	n, err := w.Read("xdata", line)
	if err != nil {
		return nil, err
	}
	if n == NBUF {
		// TODO(mpl): do something better about this
		return nil, errors.New("xdata is too long to be read in one call.")
	}
	if line[0] != '	' {
		proj := strings.Split(string(line), ":")[0]
		if !projectWord.MatchString(proj + ":") {
			return nil, errors.New("wrong clic")
		}
		return &query{
			project: proj,
			kind:    locationKeyword,
		}, nil
	}

	target := string(e.Text)
	q := new(query)
	if _, ok := guruModes[target]; ok {
		q.kind = guruKeyword
		q.mode = target
		q.where = string(e.Loc)
	} else {
		q.kind = locationKeyword
		q.where = target
	}
	elevator := ""
	for {
		// read line above
		elevator += "-"
		addr = "#" + fmt.Sprint(e.OrigQ0) + elevator + "+"
		err := w.Addr("%s", addr)
		if err != nil {
			return nil, err
		}
		_, err = w.Read("xdata", line)
		if err != nil {
			return nil, err
		}
		if line[0] != '	' {
			// found the project line
			proj := strings.Split(string(line), ":")[0]
			q.project = proj
			return q, nil
		}
	}
	return nil, errors.New("invalid search kind")
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
			q, err := buildQuery(*e)
			if err != nil {
				log.Print(err)
				continue
			}
			q.what = string(e.Arg)
			dispatchSearch(q)
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

func guru(mode, loc, scope string) error {
	args := []string{mode, loc}
	if needsScope, _ := guruModes[mode]; needsScope {
		args = []string{"-scope", strings.Join(projects[scope].GuruScope, ","), mode, loc}
	}
	cmd := exec.Command("guru", args...)
	var stderr, stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v; %v; %v; %v", err, strings.Join(args, " "), string(stderr.Bytes()), string(stdout.Bytes()))
	}
	fmt.Fprint(os.Stdout, "********\n")
	fmt.Fprintf(os.Stdout, "%s", string(stdout.Bytes()))
	fmt.Fprint(os.Stdout, "********\n")
	w.Write("body", []byte("guru "+strings.Join(args, " ")+"\n"))
	return nil
}

// TODO(mpl): use go generate to write that doc as the program doc too?

var docTxt = `
The gofinder program is an acme user interface to search through Go projects.
It uses 2-1 chording (see https://swtch.com/plan9port/man/man1/acme.html).
It uses a JSON configuration file to define project(s) to search on; see
projects-example.json for a working configuration example.

It displays, in the following order: The name of the project, to perform a
global search. The Go Guru (golang.org/x/tools/cmd/guru) modes, to perform a
guru search. The project's locations, to perform a local search. For example,
with the provided projects-example.json, the UI will look like:

	Search in: 
	-----------------------------------
	camlistore:
		callees	callers	callstack	definition	describe	freevars	implements	peers	pointsto	referrers	what	whicherrs
		/home/mpl/src/camlistore.org	/home/mpl/src/camlistore.org/vendor	/home/mpl/src/go4.org	/home/mpl/src/github.com/mpl
	-----------------------------------


The configuration file is mapped to a project type, which is defined as follows:

	type Project struct {
		// Name is the one word name describing the project, that will appear at
		// the top of the UI. One word, because chording on the name starts a
		// global search in the project. Global search means a find on all the
		// files ending with the extensions defined in Exts, looking in the
		// locations defined in Locations, excluding all the patterns defined in
		// Excluded. The results are piped to a grep for the argument that is sent
		// with the chord.
		Name      string
		// Locations defines all the locations relevant to the project, and as
		// such, they are displayed on the UI. A global search runs find through
		// all of them. A chording on one of the locations will perform a local
		// search, i.e. in the same way as a global search, except find will only
		// run through that one location.
		Locations []string
		// Exts defines the file extension patterns (regexp), that find will
		// take into account. It defaults to []string{"\.go"} otherwise.
		Exts      []string
		// Excluded defines the patterns (regexp), that find will take into
		// account to exclude from the search results.
		Excluded  []string
		// GuruScope is the scope that guru will use for the modes that need one.
		GuruScope []string
	}
`

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: gofind projects.json \n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, docTxt)
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *help {
		usage()
	}
	args := flag.Args()

	if flag.NArg() != 1 {
		usage()
	}

	// TODO(mpl): restore CLI mode when ready.

	configFile = args[0]
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
