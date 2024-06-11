FROM node:latest AS build-css
WORKDIR /build

COPY package.json package-lock.json .
RUN npm ci

COPY ./tailwind.css ./tailwind.config.js .
RUN npx tailwindcss -i ./tailwind.css -o ./output.css

FROM golang:latest AS build-go
WORKDIR /build

COPY go.mod go.sum .
RUN go mod download

COPY swim.go .
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -v -ldflags="-w -s" ./swim.go

FROM scratch 
WORKDIR /app


COPY --from=build-css /build/output.css ./public/css/main.css
COPY --from=build-go /build/swim ./swim

ENTRYPOINT ["/app/swim"]

