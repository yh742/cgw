FROM golang:1.12-alpine as builder
RUN apk update && \
    apk add --update git
WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./
WORKDIR /src
RUN go build -o ds ./cmd/ds/*.go

FROM golang:1.12-alpine
RUN apk update
RUN mkdir -p /etc/ds/
WORKDIR /app
COPY --from=builder /src/ds .
ENTRYPOINT [ "/app/ds" ]
