FROM golang:1.13.5-alpine3.10 AS godev

RUN apk update && apk add --no-cache ca-certificates && apk upgrade && apk add git

WORKDIR /trello-sync

COPY . .
ENV GO111MODULE=on
ENV GOSUMDB=off
ENV GOPROXY=direct

RUN go build -o trello-sync .

FROM alpine:3.9

COPY VERSION .

RUN apk update && apk add --no-cache ca-certificates && apk upgrade

COPY --from=godev ./trello-sync/trello-sync /trello-sync

CMD ["sh", "/trello-sync -h"]