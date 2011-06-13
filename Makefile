include $(GOROOT)/src/Make.inc
 
TARG=gofind

GOFILES=\
	c++.go		\
	fortran.go	\
	go.go				\
	main.go		\
	server.go

include $(GOROOT)/src/Make.cmd
