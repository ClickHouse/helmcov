FROM golang:1.23 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/helmcov ./cmd/helmcov

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /work
COPY --from=build /out/helmcov /usr/local/bin/helmcov
ENTRYPOINT ["/usr/local/bin/helmcov"]
