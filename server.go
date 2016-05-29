package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	//	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"

	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
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

	// TODO: this big switch is terrible. make a map instead.
	switch m.Action {
	case regex:
		findRegex(m.What, *where, exts, proj.Excluded)
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
		findGoFunc(m.What, *where, proj.Excluded)
	case goMeth:
		findGoMeth(m.What, *where, proj.Excluded)
	case goTyp:
		findGoType(m.What, *where, proj.Excluded)
	case pyFunc:
		findPyFunc(m.What, *where)
	default:
		println(m.Action, m.What)
	}
	log.Printf("********")
	w.Write("body", []byte(m.What+"\n"))
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

const exitStatusOne = "exit status 1"

//TODO: follow symlinks?
//TODO: write to acme win once we replace find and grep with native code
func findRegex(reg string, list []string, exts []string, excl []string) {
	findProcMu.Lock()
	defer findProcMu.Unlock()
	println("regex: " + reg)
	var err error
	//		pr, pw := io.Pipe()
	pr, pw, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}
	defer pr.Close()

	go func() {
		nargs := 5
		args1 := make([]string, 0, nargs+len(list))
		args1 = append(args1, "/usr/bin/find")
		args1 = append(args1, list...)
		exp := ".*(" + strings.Join(exts, "|") + ")$"
		args1 = append(args1, "-regextype", "posix-egrep", "-regex", exp)
		if excl != nil {
			for _, v := range excl {
				args1 = append(args1, "-a", "!", "-regex", v)
			}
		}
		// TODO(mpl): use an exec.Cmd, and specify Stdout as the pipe?
		fds1 := []*os.File{os.Stdin, pw, os.Stderr}
		p1, err := os.StartProcess(args1[0], args1, &os.ProcAttr{Dir: "/", Files: fds1})
		if err != nil {
			log.Fatalf("Couldn't start 'find': %v", err)
		}
		_, err = p1.Wait()
		if err != nil {
			log.Fatalf("Error with 'find': %v", err)
		}
		pw.Close()
	}()

	sc := bufio.NewScanner(pr)
	var lines []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > 9 {
			args2 := append([]string{"/bin/grep", "-E", "-n", reg}, lines...)
			lines = lines[:0]
			mu.Lock()
			go func() {
				defer mu.Unlock()
				wg.Add(1)
				defer wg.Done()
				cmd := exec.Command(args2[0], args2[1:]...)
				out, err := cmd.Output()
				if err != nil {
					// Because exit status 1 is grep simply didn't find anything.
					if !strings.Contains(err.Error(), exitStatusOne) {
						log.Fatalf("grep failed: %v, %s", err, string(err.(*exec.ExitError).Stderr))
					}
					return
				}
				fmt.Fprintf(os.Stdout, "%s", string(out))
			}()
		}
	}
	wg.Wait()
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
	if len(lines) <= 0 {
		return
	}

	args2 := append([]string{"/bin/grep", "-E", "-n", reg}, lines...)
	cmd := exec.Command(args2[0], args2[1:]...)
	out, err := cmd.Output()
	if err != nil {
		// Because exit status 1 is grep simply didn't find anything.
		if !strings.Contains(err.Error(), exitStatusOne) {
			log.Fatalf("grep failed: %v, %s", err, string(err.(*exec.ExitError).Stderr))
		}
		return
	}
	fmt.Fprintf(os.Stdout, "%s", string(out))
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
	msg := &plumb.Message{
		Src:  "gofinder",
		Dst:  "edit",
		Dir:  "/",
		Type: "text",
		Data: []byte(fullPath),
	}
	return msg.Send(port)
}
