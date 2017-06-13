# Docker image with intravatar as entrypoint
#
# Note that the project is build on the container itself,
# and that the build is not using the make file.

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
