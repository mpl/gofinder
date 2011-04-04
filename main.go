package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"goplan9.googlecode.com/hg/plan9"
	"bitbucket.org/fhs/goplumb/plumb"
)

const fortranCall = "call"
const fortranUseModule = "use"
const fortranSubroutine = "^subroutine "
const fortranModule = "^module "
const cExt = `\.h|\.c`
const fortranExt = `\.f90`
const allExt = cExt + "|" + fortranExt

//TODO: make a server of it. takes message to add/set includes, languages, etc...
var (
	fortranCallValidator = regexp.MustCompile(".*"+ fortranCall + ".*")
	fortranUseModuleValidator = regexp.MustCompile(".*"+ fortranUseModule + ".*")
//	fortranIncludes []string = []string{"/home/mpl/work/gildas-dev/kernel/", "/home/mpl/work/gildas-dev/packages/"}
//	cppIncludes []string = []string{"/home/mpl/work/casa/casacore", "/home/mpl/work/casa/active/code/include"}
	fortranIncludes []string = []string{"/home/mpl/git/iram/otf"}
//	cppIncludes []string = []string{"/home/mpl/tmp/9torrent", "/home/mpl/git/"}
	cppIncludes []string = []string{"/home/mpl/git/"}
	reg = flag.String("r", "", "regexp to search for")
	help = flag.Bool("h", false, "show this help")
)

//TODO: a more portable/native solution
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

func findFortranSubroutine(call string) {
//TODO: match the number of args of the subroutine
	findRegex(fortranSubroutine + strings.Split(call, "(", -1)[0] + `(.*`,
		fortranIncludes, fortranExt)
}

func findFortranModule(module string) {
	findRegex(fortranModule + module,
		fortranIncludes, fortranExt)
}

func findInclude(include string, list []string) {
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
				log.Fatal(err)
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
					findInclude(include, []string{fullPath})
				}
			}
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: \n\t gofinder include_path \n");
	fmt.Fprintf(os.Stderr, "\t gofinder -r regexp \n");
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
		println("regexp: "+*reg) 
		if *reg == "" {
			usage()
		}
		findRegex(*reg, cppIncludes, allExt)
		return
	}

	arg0 := flag.Args()[0]
	arg1 := ""
	// check for chording
	chorded := strings.Fields(arg0)
	if len(chorded) > 1 {
		arg0 = chorded[0]
		arg1 = chorded[1]
	} else {
		if flag.NArg() == 2 {
			arg1 = flag.Args()[1]
		}
	}
	switch {
	case fortranCallValidator.MatchString(arg0):
		findFortranSubroutine(arg1)
	case fortranUseModuleValidator.MatchString(arg0):
		findFortranModule(arg1)
	default:
		findInclude(arg0, cppIncludes)
	}
}
