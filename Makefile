# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.


GOBIN = build/bin
GO ?= latest

all:
	build/env.sh go run build/ci.go install ./client/siotchain
	build/env.sh go run build/ci.go install ./client/siotchain_cli
	@echo "Done building."

clean:
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

