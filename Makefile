include $(GOROOT)/src/Make.inc
 
TARG=gofind

GOFILES=\
	c++.go		\
	fortran.go	\
	main.go		\
	server.go

include $(GOROOT)/src/Make.cmd
