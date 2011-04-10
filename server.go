package main

import (
	"gob"
	"log"
	"net"
	"os"
	"path"
	"goplan9.googlecode.com/hg/plan9"
	"bitbucket.org/fhs/goplumb/plumb"
)

const (
	fortranZone = iota
	cppZone
	allZones
)

var (
	lang = ""
	fortranIncludes []string 
	cppIncludes []string 
	allCode []string 

//	fortranIncludes []string = []string{"/home/mpl/work/gildas-dev/kernel/", "/home/mpl/work/gildas-dev/packages/"}
//	cppIncludes []string = []string{"/home/mpl/work/casa/casacore", "/home/mpl/work/casa/active/code/include"}
//	allLocs []string = []string{"/home/mpl/work/casa/casacore", "/home/mpl/work/casa/active/code"}
)

type msg struct {
	Action int
	What string
	Where int
}

func listen() {
	ln, err := net.Listen("tcp", ":2020")
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	for {   
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("accept error: %v", err)
		}
		go serve(conn)
		log.Printf("Accepted")
	}
}

func serve(conn net.Conn) {
	var m msg
	dec := gob.NewDecoder(conn) 
	err := dec.Decode(&m)
	if err != nil {
		log.Fatal("decode error:", err)
	}
	where := &allCode
	switch m.Where {
	case fortranZone:
		where = &fortranIncludes
	case cppZone:
		where = &cppIncludes
	case allZones:
		where = &allCode
	default:
		where = getLangZone()
	}
	switch m.Action {
	case regex:
		findRegex(m.What, *where, allExt)
	case file:
		findFile(m.What, *where)
	case addLoc:
		addLocation(m.What, where)
	case listLocs:
		listLocations(*where)
	case setLang:
		setLanguage(m.What)
	default:
		println(m.Action, m.What)
	}
}

//TODO: only set to allowed values
func setLanguage(lng string) {
	lang = lng
}

func getLangZone() *[]string {
	where := &allCode
	switch lang {
	case "fortran":
		where = &fortranIncludes
	case "cpp":
		where = &cppIncludes
	default:
		where = &allCode		
	}
	return where
}

func addLocation(location string, where *[]string) {
	*where = append(*where, location)
	// update allCode as well
	if where != &allCode {
		allCode = append(allCode, location)
	}
}

func listLocations(where []string) {
	for _,s := range where {
		println(s)
	}
}

//TODO: a more portable/native solution
//TODO: follow symlinks??
func findRegex(reg string, list []string, extensions string) {
	println(reg)
	pr, pw, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	nargs := 5
	l := len(list)
	args1 := make([]string, nargs + l)
	args1[0] = "/usr/bin/find"
	for i,s := range list {
		args1[i+1] = s
	}
	args1[l+1] = "-regextype"
	args1[l+2] = "posix-egrep"
	args1[l+3] = "-regex"
	args1[l+4] = ".*(" + extensions + ")$"
	fds1 := []*os.File{os.Stdin, pw, os.Stderr}
	
	nargs = 4
	args2 := make([]string, nargs)
	args2[0] = "/usr/bin/xargs"
	args2[1] = "/bin/grep"
	args2[2] = "-nR"
	args2[3] = reg
	fds2 := []*os.File{pr, os.Stdout, os.Stderr}
	
	_, err = os.StartProcess(args1[0], args1, os.Environ(), "/", fds1)
	if err != nil {
		log.Fatal(err)
	}
	_, err = os.StartProcess(args2[0], args2, os.Environ(), "/", fds2)
	if err != nil {
		log.Fatal(err)
	}	
}


func findFile(include string, list []string) {
	for _,includeDir := range list {
		fullPath := path.Join(includeDir, include)
		_, err := os.Lstat(fullPath)
		if err == nil {
			port, err := plumb.Open("send", plan9.OWRITE)
			if err != nil {
				log.Fatal(err)
				return
			}
			defer port.Close()
			port.Send(&plumb.Msg{
				Src:  "gofinder",
				Dst:  "",
				WDir: "/",
				Kind: "text",
				Attr: map[string]string{},
				Data: []byte(fullPath),
			})
			return	
		} else {
			//search for other dirs in current dir
			currentDir, err := os.Open(includeDir, os.O_RDONLY, 0644)
			if err != nil {
				log.Print(err)
//TODO: do we remove a dir when it's bogus? for now, just ignore it
// apparently it's not ENOENT that we hit. investigate.
				continue
			}
			names, err := currentDir.Readdirnames(-1)
			if err != nil {
				log.Fatal(err)
			}
			currentDir.Close()
			var fi *os.FileInfo
			for _, name := range names {
				fullPath = path.Join(includeDir, name)
				fi, err = os.Lstat(fullPath)
				if err != nil {
					log.Fatal(err)
				}
				if fi.IsDirectory() {
					// recurse
					findFile(include, []string{fullPath})
				}
			}
		}
	}
}

