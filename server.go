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

var (
	lang = ""
	langs = make(map[string]language, 1)
)

type msg struct {
	Action int
	What string
	Where string
}

func listen() {
	ln, err := net.Listen("tcp", ":" + *port)
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
	if m.Where == "" {
		m.Where = lang
	}
	v, ok := langs[m.Where]
	if !ok {
		v = langs[all]
	}
	where := &(v.Locations)
	
	switch m.Action {
	case regex:
		findRegex(m.What, *where, v.Ext)
	case file:
		findFile(m.What, *where)
	case fortranSubroutine:
		findFortranSubroutine(m.What)
	case fortranModule:
		findFortranModule(m.What)
	case addLoc:
		addLocation(m.What, m.Where)
	case listLocs:
		listLocations(*where)
	case setLang:
		setLanguage(m.What)
	default:
		println(m.Action, m.What)
	}
}

func setLanguage(lng string) {
	_,ok := langs[lng]
	if ok {
		lang = lng
	}
}

func addLocation(location string, where string) {
	l,ok := langs[where]
	if !ok {
		log.Printf("%s not a key in langs \n", where)
		return
	}
	l.Locations = append(l.Locations, location)
	langs[where] = l
	// update allCode as well
	if where != all {
		l, ok := langs[all]
		if !ok {
			log.Fatal("%s not a key in langs \n", where)
			return
		}
		l.Locations = append(l.Locations, location)
		langs[all] = l		
	}
}

func listLocations(where []string) {
	for _,s := range where {
		println(s)
	}
}

//TODO: follow symlinks??
func findRegex(reg string, list []string, extensions string) {
	var err os.Error
	pr, pw, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	nargs := 5
	args1 := make([]string, 0, nargs + len(list))
	args1 = append(args1, "/usr/bin/find")
	args1 = append(args1, list...)
	args1 = append(args1, "-regextype", "posix-egrep", "-regex", ".*(" + extensions + ")$")
	fds1 := []*os.File{os.Stdin, pw, os.Stderr}
	
	args2 := []string{"/usr/bin/xargs", "/bin/grep", "-n", reg}
	fds2 := []*os.File{pr, os.Stdout, os.Stderr}

	p1, err := os.StartProcess(args1[0], args1, os.Environ(), "/", fds1)
	if err != nil {
		log.Fatal(err)
	}
	pw.Close()
	
	p2, err := os.StartProcess(args2[0], args2, os.Environ(), "/", fds2)
	if err != nil {
		log.Fatal(err)
	}
	pr.Close()
	
	_, err = os.Wait(p1.Pid, os.WSTOPPED)
	if err != nil {
		log.Fatal(err)
	}	
	_, err = os.Wait(p2.Pid, os.WSTOPPED)
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

