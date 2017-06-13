FROM golang:1.8

# build
WORKDIR /go/src/intravatar
COPY *.go ./
RUN go-wrapper download   # "go get -d -v ./..."
RUN go-wrapper install    # "go install -v ./..."

# run
RUN mkdir /intravatar
WORKDIR /intravatar
COPY resources resources
COPY config.ini .

ENTRYPOINT ["intravatar"]
