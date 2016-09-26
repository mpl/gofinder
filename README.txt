gofinder
========

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
