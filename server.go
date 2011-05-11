package main

import (
	"fmt"
	"gob"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
	"goplan9.googlecode.com/hg/plan9"
	"bitbucket.org/fhs/goplumb/plumb"
)

var (
	proj  = ""
	projects = make(map[string]project, 1)
	filePathValidator *regexp.Regexp
)

type msg struct {
	Action int
	What   string
	Where  string
}

func listen() {
	ln, err := net.Listen("tcp", ":"+*port)
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
		m.Where = proj
	}
	v, ok := projects[m.Where]
	if !ok {
		v = projects[all]
	}
	where := &(v.Locations)
	
	switch m.Action {
	case regex:
		findRegex(m.What, *where, v.Exts)
	case file:
		if !filePathValidator.MatchString(m.What) {
			patternTofileName(m.What, *where)
		}	else {	
			openFile(m.What, *where)
		}
	case cppMethod:
		findCppMethod(m.What)
	case cppClassMethod:
		findCppClassMethod(m.What)		
	case fortranSubroutine:
		findFortranSubroutine(m.What)
	case fortranModule:
		findFortranModule(m.What)
	case listLocs:
		listLocations(*where)
	case setProj:
		setProject(m.What)
	default:
		println(m.Action, m.What)
	}
}

//TODO: factorize
func patternTofileName(what string, where []string) {
	// assume it's a class/package/etc name and try to find what's the most usual guess depending on the language
	// if no project is set, just go through all of them until there's a match
	if proj ==  "" {
		for _,v := range projects {
			for _,s := range v.Exts {
				ext := strings.Replace(s, "\\", "", -1) 
				err := openFile(what + ext, v.Locations)
				if err == nil {
					return
				}
			}				
		}
	} else {
		v, ok := projects[proj]
		if ok {
			for _,s := range v.Exts {
				ext := strings.Replace(s, "\\", "", -1) 
				openFile(what + ext, where)
			}
		}
	}
}

func setProject(prj string) {
	_, ok := projects[prj]
	if ok {
		proj = prj
	}
}

// it's so easy to just add one to the json file and restart the server that I don't think this functionality is worth keeping/maintaining. we'll see.
func addLocation(location string, where string) {
	p, ok := projects[where]
	if !ok {
		log.Printf("%s not a key in projects \n", where)
		return
	}
	p.Locations = append(p.Locations, location)
	projects[where] = p
	// update allCode as well
	if where != all {
		p, ok := projects[all]
		if !ok {
			log.Fatal("%s not a key in projects \n", where)
			return
		}
		p.Locations = append(p.Locations, location)
		projects[all] = p
	}
}

func listLocations(where []string) {
	for _, s := range where {
		println(s)
	}
}

//TODO: follow symlinks?
func findRegex(reg string, list []string, exts []string) {
	var err os.Error
	pr, pw, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	nargs := 5
	args1 := make([]string, 0, nargs+len(list))
	args1 = append(args1, "/usr/bin/find")
	args1 = append(args1, list...)
	exp := ".*("+ strings.Join(exts, "|") + ")$"
	args1 = append(args1, "-regextype", "posix-egrep", "-regex", exp)
	fds1 := []*os.File{os.Stdin, pw, os.Stderr}

	args2 := []string{"/usr/bin/xargs", "/bin/grep", "-n", reg}
	fds2 := []*os.File{pr, os.Stdout, os.Stderr}

	p1, err := os.StartProcess(args1[0], args1, &os.ProcAttr{Dir: "/", Files: fds1})
	if err != nil {
		log.Fatal(err)
	}
	pw.Close()

	p2, err := os.StartProcess(args2[0], args2, &os.ProcAttr{Dir: "/", Files: fds2})
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

func findFile(relPath string, list []string) string {
	//in case chording (or voluntary input) gave us a full path already
	if relPath[0] == '/' {
		return relPath
	}
	for _, includeDir := range list {
		fullPath := path.Join(includeDir, relPath)
		_, err := os.Lstat(fullPath)
		if err == nil {
			return fullPath
		} else {
			//search for other dirs in current dir
			currentDir, err := os.Open(includeDir)
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
					temp := findFile(relPath, []string{fullPath})
					if temp != "" {
						return temp
					}
				}
			}
		}
	}
	return ""
}

func openFile(relPath string, list []string) os.Error {
	fullPath := findFile(relPath, list)
	if fullPath == "" {
		return os.ENOENT
	}
	if *noplumb {
		fmt.Println(fullPath)
		return nil
	}
	port, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		log.Fatal(err)
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
	return nil	
}
