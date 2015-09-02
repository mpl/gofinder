gofinder
========

The gofinder program is an acme user interface to look for include files, specific patterns, and regexps.

You need a json config file to define the projects where gofind will look; see projects-example.json for the expected fields.

The resulting acme ui will look like the following.

Search in: 
-----------------------------------
proj1:
	c++:	staticMethod	staticMember	include	all
	/home/glenda/foo	/home/glenda/bar
proj2:
	fortran:	module	subroutine	function	all
	go:	package	func	all
	/home/glenda/foobar	/home/glenda/foobarbaz	/home/glenda/go
camlistore:
	go:	package	func	type	method	all
	/home/mpl/gocode/src/camlistore.org	/home/mpl/go	/home/mpl/gocode/src/github.com/mpl	/home/mpl/gocode/src/bitbucket.org/mpl	/home/mpl/git/go
	.*zembed_.*
-----------------------------------

All searches are 2-1 chords: one first selects with button 1 the queried terms in the code, and one then presses and holds button 2 on one of the words of this ui and presses button 1 while still holding button 2.

For each project, each language and its possible search methods are displayed, as well as the possible locations.
For example, say the fortran module named "foo" is used and defined in proj2. Then selecting foo in the code and chording it 2-1 on the "module" word on the fortran line of proj2 will try and find the location of this module definition. 

The "all" keyword/command will trigger a regexp search (only in the relevant files for the corresponding language - this is hardcoded) of whatever is chorded 2-1 to it.

Same behaviour for the locations, except the search will apply to all the relevant files of the project (defined by the "Exts" field) in the chorded location.

The "Excluded" field (regexp) obsiously indicates what should be excluded from the search.

Limitations:
-It does not do chained calls. But I found I don't really need it, at least with Go.

-The underlying searches rely on find and grep. They should be replaced with native go code.
