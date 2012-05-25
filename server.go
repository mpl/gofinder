package main

import (
	"bitbucket.org/fhs/goplumb/plumb"
	"code.google.com/p/goplan9/plan9"
	"encoding/gob"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"strings"
)

var (
	globalProj        = ""
	projects          map[string]project
	filePathValidator *regexp.Regexp
)

type msg struct {
	Action int
	What   string
	Where  string
}

func listen(c chan int) {
	ln, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Printf("listen error: %v", err)
		c <- 1
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			c <- 1
			return
		}
		go serve(conn)
		log.Printf("********")
		log.Printf("Accepted")
	}
}

func serve(conn net.Conn) {
	defer conn.Close()
	var m msg
	dec := gob.NewDecoder(conn)
	err := dec.Decode(&m)
	if err != nil {
		log.Fatal("decode error:", err)
	}
	var where *[]string
	var proj project
	var exts []string
	var ok bool
	if !strings.Contains(m.Where, ":") {
		log.Printf("incomplete m.Where message: %s \n", m.Where)
		return
	}
	whereSplit := strings.Split(m.Where, ":")
	proj, ok = projects[whereSplit[0]]
	if !ok {
		log.Print("not a project name: %s \n", whereSplit[0])
		return
	}
	sub := whereSplit[1]
	switch sub {
	case all:
		where = &(proj.Locations)
		exts = proj.Exts
	case location:
		where = &[]string{whereSplit[2]}
		exts = proj.Exts
	default:
		// it's a lang -> search everywhere but only with exts of the lang
		where = &(proj.Locations)
		exts, ok = allExts[sub]
		if !ok {
			log.Printf("%s not a key in allExts\n", sub)
			return
		}
	}

	switch m.Action {
	case regex:
		findRegex(m.What, *where, exts)
	case file:
		if !filePathValidator.MatchString(m.What) {
			patternTofileName(m.What, *where)
		} else {
			openFile(m.What, *where, false)
		}
	case cppInc:
		openFile(m.What, *where, false)
	case cppClassMeth:
		findCppClassMethod(m.What, *where)
	case cppClassMemb:
		findCppClassMember(m.What, *where)
	case fortFunc:
		findFortranFunction(m.What, *where)
	case fortMod:
		findFortranModule(m.What, *where)
	case fortSub:
		findFortranSubroutine(m.What, *where)
	case fortType:
		findFortranType(m.What, *where)
	case goPack:
		openFile(cleanGoPackageLine(m.What), *where, true)
	case goFunc:
		findGoFunc(m.What, *where)
	case goMeth:
		findGoMeth(m.What, *where)
	case goTyp:
		findGoType(m.What, *where)
	case pyFunc:
		findPyFunc(m.What, *where)
	default:
		println(m.Action, m.What)
	}
}

//TODO: factorize
func patternTofileName(what string, where []string) {
	// assume it's a class/package/etc name and try to find what's the most usual guess depending on the language
	// if no project is set, just go through all of them until there's a match
	if globalProj == "" {
		for _, v := range projects {
			for _, s := range v.Exts {
				ext := strings.Replace(s, "\\", "", -1)
				err := openFile(what+ext, v.Locations, false)
				if err == nil {
					return
				}
			}
		}
		//TODO: probably remove that case later
	} else {
		v, ok := projects[globalProj]
		if ok {
			for _, s := range v.Exts {
				ext := strings.Replace(s, "\\", "", -1)
				openFile(what+ext, where, false)
			}
		}
	}
}

//TODO: follow symlinks?
//TODO: write to acme win once we replace find and grep with native code
func findRegex(reg string, list []string, exts []string) {
	println("regex: " + reg)
	var err error
	pr, pw, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	nargs := 5
	args1 := make([]string, 0, nargs+len(list))
	args1 = append(args1, "/usr/bin/find")
	args1 = append(args1, list...)
	exp := ".*(" + strings.Join(exts, "|") + ")$"
	args1 = append(args1, "-regextype", "posix-egrep", "-regex", exp)
	fds1 := []*os.File{os.Stdin, pw, os.Stderr}

	args2 := []string{"/usr/bin/xargs", "/bin/grep", "-E", "-n", reg}
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

	_, err = p1.Wait()
	if err != nil {
		log.Fatal(err)
	}
	_, err = p2.Wait()
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
			var fi os.FileInfo
			for _, name := range names {
				fullPath = path.Join(includeDir, name)
				fi, err = os.Lstat(fullPath)
				if err != nil {
					log.Fatal(err)
				}
				if fi.IsDir() {
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

func findDir(relPath string, list []string) string {
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
			var fi os.FileInfo
			newdir := ""
			for _, name := range names {
				newdir = path.Join(includeDir, name)
				fi, err = os.Lstat(newdir)
				if err != nil {
					log.Fatal(err)
				}
				if fi.IsDir() {
					fullPath = path.Join(newdir, relPath)
					fi, err = os.Lstat(fullPath)
					if err == nil {
						return fullPath
					}
					// recurse
					temp := findDir(relPath, []string{newdir})
					if temp != "" {
						return temp
					}
				}
			}
		}
	}
	return ""
}

func openFile(relPath string, list []string, isDir bool) error {
	fullPath := ""
	if isDir {
		fullPath = findDir(relPath, list)
	} else {
		fullPath = findFile(relPath, list)
	}
	if fullPath == "" {
		return os.ErrNotExist
	}
	port, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()
	port.Send(&plumb.Msg{
		Src:  "gofinder",
		Dst:  "edit",
		WDir: "/",
		Kind: "text",
		Attr: map[string]string{},
		Data: []byte(fullPath),
	})
	return nil
}
