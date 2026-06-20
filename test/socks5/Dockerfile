FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum main.go ./
RUN go mod download && CGO_ENABLED=0 go build -o /socks5 .

FROM alpine:3.20
COPY --from=build /socks5 /usr/local/bin/socks5
EXPOSE 1080
CMD ["/usr/local/bin/socks5"]
