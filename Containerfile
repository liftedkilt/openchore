FROM docker.io/library/golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /openchore cmd/server/main.go

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /openchore /usr/local/bin/openchore
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

VOLUME /data
ENV DB_PATH=/data/openchore.db
ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["entrypoint.sh"]
