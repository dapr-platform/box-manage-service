FROM golang:1.23.1-alpine3.19 AS build
RUN echo "https://mirrors.aliyun.com/alpine/v3.19/main" > /etc/apk/repositories && \
    echo "https://mirrors.aliyun.com/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache git upx \
    && rm -rf /var/cache/apk/* \
    && rm -rf /root/.cache \
    && rm -rf /tmp/*
RUN mkdir /app
WORKDIR /app
COPY go.mod go.sum ./
ENV GOSUMDB=off
RUN go mod download
COPY . .
# upx 压缩级别可通过 --build-arg UPX_LEVEL=N 覆盖，默认 -3（快速压缩，-9 太慢省不了多少）
ARG UPX_LEVEL=3
RUN go build -ldflags "-s -w" -o box-manage-service . && upx -${UPX_LEVEL} box-manage-service

FROM alpine:3.19
RUN echo "https://mirrors.aliyun.com/alpine/v3.19/main" > /etc/apk/repositories && \
    echo "https://mirrors.aliyun.com/alpine/v3.19/community" >> /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata ffmpeg \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && apk del tzdata \
    && rm -rf /var/cache/apk/* \
    && rm -rf /root/.cache \
    && rm -rf /tmp/*

RUN mkdir -p /app/data/video /app/data/models /app/data/uploads/packages /app/temp

WORKDIR /app
COPY --from=build /app/box-manage-service .

RUN ffmpeg -version && ffprobe -version

ENV VIDEO_FFMPEG_PATH=ffmpeg
ENV VIDEO_FFPROBE_PATH=ffprobe

EXPOSE 80
CMD ["./box-manage-service"]