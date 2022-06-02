VERSION 0.6
FROM golang:1.18-bullseye
WORKDIR /decoder

deps:
  COPY go.mod go.sum .
  RUN go mod download
  SAVE IMAGE --cache-hint

fdkaac:
  FROM ghcr.io/izumisy/fdkaac:latest
  SAVE ARTIFACT /fdkaac-include
  SAVE ARTIFACT /fdkaac-lib

build:
  FROM +deps
  COPY +fdkaac/fdkaac-include /usr/include/fdk-aac
  COPY +fdkaac/fdkaac-lib /usr/lib/fdk-aac
  COPY . .
  RUN go build -o build/decoder decoder.go
  SAVE ARTIFACT build/decoder AS LOCAL build/decoder
