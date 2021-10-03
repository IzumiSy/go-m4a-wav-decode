.PHONY: build dep build

dep:
	git submodule update --init --recursive

setup: dep
	cd lib/go-fdkaac && ./build.sh && cd -

build: main.go
	go build main.go
