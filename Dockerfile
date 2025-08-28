FROM golang:1.23.1-alpine3.19 AS build
RUN  apk add --no-cache git upx \
    && rm -rf /var/cache/apk/* \
    && rm -rf /root/.cache \
    && rm -rf /tmp/*
RUN mkdir /app
WORKDIR /app
COPY go.mod .
COPY go.sum .
ENV GOSUMDB=off
RUN go mod tidy
COPY . .
RUN go build -ldflags "-s -w" -o  box-manage-service && upx -9 box-manage-service

FROM alpine:3.19
RUN  apk add --no-cache ca-certificates tzdata ffmpeg \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && apk del tzdata \
    && rm -rf /var/cache/apk/* \
    && rm -rf /root/.cache \
    && rm -rf /tmp/*

RUN mkdir -p /app/data/video /app/data/models /app/temp

WORKDIR /app
COPY --from=build /app/box-manage-service .

# 验证FFmpeg和FFprobe是否安装成功
RUN ffmpeg -version && ffprobe -version

# 设置FFmpeg相关环境变量
ENV VIDEO_FFMPEG_PATH=ffmpeg
ENV VIDEO_FFPROBE_PATH=ffprobe

EXPOSE 80
CMD ["./box-manage-service"]