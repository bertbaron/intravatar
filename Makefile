all: clean linux darwin arm windows

ifneq (,$(shell which gtar 2>/dev/null))
TAR ?= gtar
else
TAR ?= tar
endif

RESOURCES=resources config.ini LICENSE
TARCMD=${TAR} --transform 's|^|intravatar/|' -czf

init:
	go get -v -d
	go test

linux: init
	GOOS=linux GOARCH=amd64 go build -o intravatar . ; \
	${TARCMD} intravatar-linux-amd64.tar.gz intravatar ${RESOURCES}

darwin: init
	GOOS=darwin GOARCH=amd64 go build -o intravatar . ; \
	${TARCMD} intravatar-darwin-amd64.tar.gz intravatar ${RESOURCES}

arm: init
	GOOS=linux GOARCH=arm go build -o intravatar . ; \
	${TARCMD} intravatar-linux-arm.tar.gz intravatar ${RESOURCES}

windows: init
	GOOS=windows GOARCH=amd64 go build -o intravatar.exe . ; \
	${TARCMD} intravatar-windows-amd64.tar.gz intravatar.exe ${RESOURCES} ; \
	(   rm -rf tmp \
	 && mkdir tmp \
	 && cd tmp \
	 && tar -xf ../intravatar-windows-amd64.tar.gz \
	 && unix2dos intravatar/config.ini intravatar/LICENSE \
	 && zip -r ../intravatar-windows-amd64 intravatar)

clean:
	-rm -rf tmp
	-rm -f intravatar
	-rm -f *.exe
	-rm -f *.gz
	-rm -f *.zip
