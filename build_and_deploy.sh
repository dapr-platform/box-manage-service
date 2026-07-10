#!/bin/bash

# 一键打包部署脚本
# 1. 本机交叉编译验证 (Linux AMD64)
# 2. 构建 Docker 镜像 (linux/amd64)
# 3. 推送到镜像仓库
# 4. 远程 SSH 更新服务并查看日志

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  box-manage-service 一键打包部署脚本${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# 项目根目录（脚本所在目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 镜像名称（可通过环境变量覆盖）
IMAGE_NAME="${IMAGE_NAME:-registry.cn-hangzhou.aliyuncs.com/daprplatform/box-manage-service}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

# 远程部署服务器（可通过环境变量覆盖）
SSH_HOST="${SSH_HOST:-182.92.117.41}"
SSH_PORT="${SSH_PORT:-40400}"
SSH_USER="${SSH_USER:-stec}"
SSH_PASS="${SSH_PASS:-stec123456@}"
REMOTE_DIR="${REMOTE_DIR:-/root/box-manager/docker-compose}"
REMOTE_UPDATE_SCRIPT="${REMOTE_UPDATE_SCRIPT:-./update-box-manage-service.sh}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-local_box_center}"
SERVICE_NAME="${SERVICE_NAME:-box-manage-service}"
UPX_LEVEL="${UPX_LEVEL:-3}"

# 步骤1: 本机交叉编译验证（提前暴露 Go 编译错误）
echo -e "${YELLOW}[步骤 1/4] 本机交叉编译验证 Linux AMD64...${NC}"
GOOS=linux
GOARCH=amd64

# 注入构建信息
BUILD_TIME=$(date +"%Y-%m-%d %H:%M:%S")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=${VERSION:-"dev"}
LDFLAGS="-s -w -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}' -X 'main.Version=${VERSION}'"

echo "正在为 $GOOS/$GOARCH 平台构建..."
echo "  BuildTime  = ${BUILD_TIME}"
echo "  GitCommit  = ${GIT_COMMIT}"
echo "  Version    = ${VERSION}"

# 生成API文档
generate_docs() {
    echo "生成API文档..."
    if command -v swag &> /dev/null; then
        swag init -g main.go -o docs/
        if [ $? -eq 0 ]; then
            echo "API文档生成完成"
        else
            echo "API文档生成失败，但继续构建"
        fi
    else
        echo "swag未安装，跳过文档生成"
        echo "安装swag: go install github.com/swaggo/swag/cmd/swag@latest"
    fi
}

generate_docs

GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
    -ldflags "${LDFLAGS}" \
    -o box-manage-service-${GOOS}-${GOARCH} \
    main.go

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Go 构建成功: box-manage-service-${GOOS}-${GOARCH}${NC}"
    chmod +x box-manage-service-${GOOS}-${GOARCH}
    BIN_SIZE=$(du -h "box-manage-service-${GOOS}-${GOARCH}" | awk '{print $1}')
    echo -e "${GREEN}✓ 产物大小: ${BIN_SIZE}${NC}"
else
    echo -e "${RED}✗ Go 构建失败${NC}"
    exit 1
fi

echo ""

# 步骤2: 构建 Docker 镜像
echo -e "${YELLOW}[步骤 2/4] 构建 Docker 镜像...${NC}"

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}✗ Docker 未运行，请启动 Docker 后再试${NC}"
    exit 1
fi

# 检查 buildx 是否可用
if ! docker buildx version > /dev/null 2>&1; then
    echo -e "${RED}✗ docker buildx 不可用，请升级 Docker 或启用 BuildKit${NC}"
    exit 1
fi

echo "构建镜像: ${FULL_IMAGE}"
docker buildx build --platform linux/amd64 -t "${FULL_IMAGE}" -f Dockerfile.local --load .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Docker 镜像构建成功${NC}"
else
    echo -e "${RED}✗ Docker 镜像构建失败${NC}"
    exit 1
fi

echo ""

# 步骤3: 推送 Docker 镜像
echo -e "${YELLOW}[步骤 3/4] 推送 Docker 镜像到仓库...${NC}"

# 通过 SKIP_PUSH=1 可跳过推送（仅本地构建）
if [ "${SKIP_PUSH}" = "1" ]; then
    echo -e "${YELLOW}⏭  SKIP_PUSH=1，跳过推送步骤${NC}"
else
    echo "推送镜像: ${FULL_IMAGE}"
    docker push "${FULL_IMAGE}"

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Docker 镜像推送成功${NC}"
    else
        echo -e "${RED}✗ Docker 镜像推送失败，请检查仓库权限（docker login）${NC}"
        exit 1
    fi
fi

echo ""

# 步骤4: 远程更新服务
echo -e "${YELLOW}[步骤 4/4] 远程更新服务...${NC}"

# 检查 sshpass 是否可用
if ! command -v sshpass &> /dev/null; then
    echo -e "${RED}✗ sshpass 未安装，请先安装: brew install hudochenkov/sshpass/sshpass${NC}"
    echo -e "${YELLOW}⏭  跳过远程更新步骤${NC}"
else
    # 通过 SKIP_REMOTE=1 可跳过远程更新
    if [ "${SKIP_REMOTE}" = "1" ]; then
        echo -e "${YELLOW}⏭  SKIP_REMOTE=1，跳过远程更新步骤${NC}"
    else
        echo "远程服务器: ${SSH_USER}@${SSH_HOST}:${SSH_PORT}"
        echo "目标目录:   ${REMOTE_DIR}"
        echo "更新脚本:   ${REMOTE_UPDATE_SCRIPT}"
        echo ""

        # 连接远程服务器并执行更新流程
        #  1. sudo su 切换到 root
        #  2. 进入 docker-compose 目录
        #  3. 执行更新脚本
        #  4. 查看服务日志（tail 最近 60 行，持续 10 秒以便观察启动情况）
        sshpass -p "${SSH_PASS}" ssh \
            -o StrictHostKeyChecking=no \
            -o UserKnownHostsFile=/dev/null \
            -o LogLevel=ERROR \
            -p "${SSH_PORT}" \
            "${SSH_USER}@${SSH_HOST}" \
            "echo '${SSH_PASS}' | sudo -S su -c 'cd ${REMOTE_DIR} && ${REMOTE_UPDATE_SCRIPT} && echo && echo \"========== 服务日志（最近 120 行）==========\" && docker-compose -p ${COMPOSE_PROJECT} logs --tail 120 ${SERVICE_NAME}'"

        REMOTE_EXIT=$?
        if [ ${REMOTE_EXIT} -eq 0 ]; then
            echo ""
            echo -e "${GREEN}✓ 远程服务更新成功${NC}"
        else
            echo ""
            echo -e "${RED}✗ 远程服务更新失败 (exit code: ${REMOTE_EXIT})${NC}"
            exit ${REMOTE_EXIT}
        fi
    fi
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  一键打包部署完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "构建产物:"
echo "  - 可执行文件: box-manage-service-${GOOS}-${GOARCH}"
echo "  - Docker镜像: ${FULL_IMAGE}"
echo ""
echo "部署命令示例:"
echo "  docker run -d --name box-manage-service \\"
echo "    -p 80:80 \\"
echo "    --env-file .env \\"
echo "    -v \$(pwd)/data:/app/data \\"
echo "    ${FULL_IMAGE}"
echo ""
echo "可用环境变量:"
echo "  IMAGE_NAME       镜像仓库地址 (默认: registry.cn-hangzhou.aliyuncs.com/daprplatform/box-manage-service)"
echo "  IMAGE_TAG        镜像标签 (默认: latest)"
echo "  VERSION          版本号注入到二进制 (默认: dev)"
echo "  SKIP_PUSH=1      仅构建不推送"
echo "  SKIP_REMOTE=1    跳过远程更新步骤"
echo "  SSH_HOST         远程服务器地址 (默认: ${SSH_HOST})"
echo "  SSH_PORT         远程服务器端口 (默认: ${SSH_PORT})"
echo "  SSH_USER         远程服务器用户 (默认: ${SSH_USER})"
echo "  SSH_PASS         远程服务器密码 (默认: 已设置)"
echo "  REMOTE_DIR       远程更新目录 (默认: ${REMOTE_DIR})"
echo "  COMPOSE_PROJECT  docker-compose 项目名 (默认: ${COMPOSE_PROJECT})"
echo "  SERVICE_NAME     服务名称 (默认: ${SERVICE_NAME})"
echo "  UPGRADE_PACKAGE_UPLOAD_DIR 升级包存放目录 (默认: ./data/uploads/packages)"
echo "  UPX_LEVEL        二进制压缩级别 1-9 (默认: 3，越大越慢)"
echo ""
