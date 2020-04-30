# Docker image with intravatar as entrypoint
#
# This is a multi-stage dockerfile. The app is
# build in the first stage
# and then copied onto a fresh alpine image
#
# Note that the make file is not used

# Builder image
FROM golang:1.14-alpine as builder

WORKDIR /go/src/github.com/bertbaron/intravatar
COPY *.go ./
RUN apk add --no-cache git
RUN go get -v -d
RUN CGO_ENABLED=0 go test
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o intravatar .
# Target image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /intravatar
COPY --from=builder /go/src/github.com/bertbaron/intravatar/intravatar .
COPY resources resources
COPY config.ini .

ENTRYPOINT ["./intravatar"]
