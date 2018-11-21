package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	projects          map[string]Project
	filePathValidator *regexp.Regexp
)

type msg struct {
	Action  int
	Project string
	What    string
	Where   string
}

type response struct {
	body string // JSON string
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

func serveProjects(conn net.Conn) error {
	return json.NewEncoder(conn).Encode(projects)
}

func serve(conn net.Conn) {
	defer conn.Close()
	var m msg
	dec := gob.NewDecoder(conn)
	err := dec.Decode(&m)
	if err != nil {
		log.Fatal("decode error:", err)
	}
	if m.Action == doGetProjects {
		if err := serveProjects(conn); err != nil {
			log.Print(err)
		}
		return
	}
	proj, ok := projects[m.Project]
	if !ok {
		log.Print("not a project name: %s \n", m.Project)
		return
	}
	var where *[]string
	if m.Where != "" {
		where = &[]string{m.Where}
	} else {
		where = &(proj.Locations)
	}
	exts := proj.Exts
	if len(exts) == 0 {
		exts = []string{`\.go`}
	}

	// TODO(mpl): restore whatever case file was?
	switch m.Action {
	case regex:
		findRegex(m.What, *where, exts, proj.Excluded)
	case file:
		if !filePathValidator.MatchString(m.What) {
			patternTofileName(m.What, *where)
		} else {
			openFile(m.What, *where, false)
		}
	default:
		println(m.Action, m.What)
	}
	log.Printf("********")
	w.Write("body", []byte(m.What+"\n"))
}

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

// TODO(mpl): follow symlinks?
// TODO(mpl): write to acme win once we replace find and grep with native code
func findRegex(reg string, list []string, exts []string, excl []string) {
	findProcMu.Lock()
	defer findProcMu.Unlock()
	var err error
	pr, pw := io.Pipe()
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
		cmd := exec.Command(args1[0], args1[1:]...)
		cmd.Stdout = pw
		if err := cmd.Run(); err != nil {
			log.Fatalf("find failed: %v, %s", err, string(err.(*exec.ExitError).Stderr))
		}
		pw.Close()
	}()

	sc := bufio.NewScanner(pr)
	var lines []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	for sc.Scan() {
		killGrepMu.Lock()
		if killGrep {
			// TODO(mpl): kill last grep? it's only running over 10 files at max, so kindof ok to let it finish.
			lines = nil
			// Consume all of pr to let find finish gracefully
			if _, err := io.Copy(ioutil.Discard, pr); err != nil {
				log.Fatal(err)
			}
			killGrep = false
			killGrepMu.Unlock()
			break
		}
		killGrepMu.Unlock()
//		println("LINE: ", sc.Text())
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

	// TODO(mpl): refactor as func
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
