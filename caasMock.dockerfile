FROM golang:1.13-alpine as builder
RUN apk update && \
    apk add --update git
WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./
WORKDIR /src
RUN go build -o caasMock ./cmd/caasMock/main.go

FROM golang:1.13-alpine
RUN apk update
WORKDIR /app
COPY --from=builder /src/caasMock .
ENTRYPOINT [ "/app/caasMock" ]
