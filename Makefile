all: clean linux darwin arm windows

RESOURCES=resources config.ini LICENSE

init:
	go get ./...

linux: init
	GOOS=linux GOARCH=amd64 go build -o intravatar . ; \
	tar --transform 's|^|intravatar/|' -czf intravatar-linux-amd64.tar.gz intravatar ${RESOURCES}

darwin: init
	GOOS=darwin GOARCH=amd64 go build -o intravatar . ; \
	tar --transform 's|^|intravatar/|' -czf intravatar-darwin-amd64.tar.gz intravatar ${RESOURCES}

arm: init
	GOOS=linux GOARCH=arm go build -o intravatar . ; \
	tar --transform 's|^|intravatar/|' -czf intravatar-linux-arm.tar.gz intravatar ${RESOURCES}

windows: init
	GOOS=windows GOARCH=amd64 go build -o intravatar.exe . ; \
	tar --transform 's|^|intravatar/|' -czf intravatar-windows-amd64.tar.gz intravatar.exe ${RESOURCES} ; \
	(rm -rf tmp && mkdir tmp && cd tmp && tar -xf ../intravatar-windows-amd64.tar.gz && zip -r ../intravatar-windows-amd64 intravatar)

clean:
	-rm -rf tmp
	-rm -f intravatar
	-rm -f *.exe
	-rm -f *.gz
	-rm -f *.zip
