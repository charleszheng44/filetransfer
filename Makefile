all: ftr

bin/ftr: main.go go.mod
	@mkdir -p bin
	go build -o bin/ftr main.go

ftr: bin/ftr

clean:
	rm -rf bin

.PHONY: ftr clean all
