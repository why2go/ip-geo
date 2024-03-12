FROM golang:1.21-alpine AS builder
ENV GOPROXY https://goproxy.cn,direct
ENV GOPRIVATE='xxxxx'
ENV CGO_ENABLED=0
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk update --no-cache
WORKDIR /build
ADD go.mod .
ADD go.sum .
RUN go mod download
COPY . .
COPY ./etc /app/etc
RUN go build -ldflags="-s -w" -o /app/ip_geo .

FROM alpine:3.19
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
&& apk update && apk upgrade && apk add curl
WORKDIR /app
COPY --from=builder /app/ip_geo /app/ip_geo

ENTRYPOINT [ "./ip_geo"]
