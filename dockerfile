FROM golang:1.13-alpine as builder
RUN apk update && \
    apk add --update git
WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./
WORKDIR /src
RUN go build -o cgw ./cmd/main/main.go

FROM golang:1.13-alpine
RUN apk update
RUN mkdir -p /etc/cgw/
WORKDIR /app
COPY --from=builder /src/cgw .
ENTRYPOINT [ "/app/cgw" ]
