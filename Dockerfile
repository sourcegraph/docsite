FROM golang:alpine as builder
ENV GO111MODULE on
COPY . $GOPATH/src/github.com/sourcegraph/docsite
WORKDIR $GOPATH/src/github.com/sourcegraph/docsite
RUN apk add --no-cache ca-certificates
RUN apk add --no-cache git && go get -d ./cmd/docsite && apk del git
RUN env GOOS=linux GARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/docsite ./cmd/docsite
RUN adduser -D -g '' appuser

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/bin/docsite /go/bin/docsite
USER appuser
EXPOSE 5080
ENTRYPOINT ["/go/bin/docsite"]
