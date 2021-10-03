.PHONY: dep

LIB_FDKAAC_OBJ = lib/go-fdkaac/fdkaac-lib-objs/lib/libfdk-aac.a

decoder: $(LIB_FDKAAC_OBJ)
	go build decoder.go

$(LIB_FDKAAC_OBJ): dep
	cd lib/go-fdkaac \
		&& ./build.sh \
		&& cd -

dep:
	git submodule update --init --recursive
