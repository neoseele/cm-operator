FROM golang:1.15-alpine3.12 as builder
RUN apk --no-cache add git

RUN mkdir /prom-operator
WORKDIR /prom-operator
COPY go.mod .
COPY go.sum .

# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download
# COPY the source code as the last step so the `go mod download` layer can be reused
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/prom-operator

FROM alpine:3.12
RUN apk --no-cache add curl
COPY --from=builder /go/bin/prom-operator /prom-operator
ENTRYPOINT ["/prom-operator"]
