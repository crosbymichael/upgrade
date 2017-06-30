FROM golang:1.8
RUN go get github.com/lk4d4/vndr
WORKDIR /go/src/github.com/crosbymichael/upgrade
COPY vendor.conf .
RUN vndr -whitelist '.*'
COPY . .
