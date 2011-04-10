include $(GOROOT)/src/Make.inc
 
TARG=gofind

GOFILES=\
	fortran.go \
	main.go	\
	server.go

include $(GOROOT)/src/Make.cmd
