.PHONY: build dep

dep:
	git submodule add -f https://github.com/winlinvip/fdk-aac.git vendor/github.com/IzumiSy/go-fdkaac/fdk-aac-lib

build: dep
	sudo apt install autoconf libtool
	cd vendor/github.com/IzumiSy/go-fdkaac/fdk-aac-lib && \
		bash autogen.sh && ./configure --prefix=`pwd`/objs && make && make install && cd -
