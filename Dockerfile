FROM alpine:3.6 as alpine

RUN apk add -U --no-cache ca-certificates


FROM node:latest AS build-css
WORKDIR /build

COPY package.json package-lock.json .
RUN npm ci

COPY ./tailwind.css ./tailwind.config.js .
COPY ./templates ./templates
RUN npx tailwindcss -i ./tailwind.css -o ./output.css

FROM golang:latest AS build-go
WORKDIR /build

COPY go.mod go.sum .
RUN go mod download

COPY swim.go .
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -v -ldflags="-w -s" ./swim.go

FROM scratch
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /app

COPY ./public ./public
COPY ./templates ./templates

COPY --from=build-css /build/output.css ./public/css/main.css
COPY --from=build-go /build/swim ./swim

EXPOSE 80
ENV HOST="0.0.0.0"
ENV PORT=80

ENTRYPOINT ["./swim"]
