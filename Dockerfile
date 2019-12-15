FROM golang:1.13 AS godev

WORKDIR /trello-sync

COPY . .
ENV GO111MODULE=on
ENV GOSUMDB=off
ENV GOPROXY=direct

RUN go build -o trello-sync .

FROM alpine:3.9

RUN apk update && apk add --no-cache ca-certificates && apk upgrade

COPY --from=godev ./trello-sync .

CMD ["sh", "./trello-sync sync"]