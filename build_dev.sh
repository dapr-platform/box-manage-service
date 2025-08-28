#!/bin/bash

# AI盒子管理系统后端服务开发构建脚本
# 用于本地开发环境的快速构建和运行

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 函数定义
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查Go环境
check_go_env() {
    log_info "检查Go环境..."
    if ! command -v go &> /dev/null; then
        log_error "Go未安装或不在PATH中"
        exit 1
    fi
    
    GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+\.[0-9]\+')
    log_info "Go版本: $GO_VERSION"
}

# 安装依赖
install_deps() {
    log_info "安装Go依赖..."
    go mod tidy
    if [ $? -eq 0 ]; then
        log_success "依赖安装完成"
    else
        log_error "依赖安装失败"
        exit 1
    fi
}

# 生成API文档
generate_docs() {
    log_info "生成API文档..."
    if command -v swag &> /dev/null; then
        swag init -g main.go -o docs/
        if [ $? -eq 0 ]; then
            log_success "API文档生成完成"
        else
            log_warning "API文档生成失败，但继续构建"
        fi
    else
        log_warning "swag未安装，跳过文档生成"
        log_info "安装swag: go install github.com/swaggo/swag/cmd/swag@latest"
    fi
}

# 代码检查
lint_code() {
    log_info "执行代码检查..."
    if command -v golangci-lint &> /dev/null; then
        golangci-lint run
        if [ $? -eq 0 ]; then
            log_success "代码检查通过"
        else
            log_warning "代码检查发现问题，但继续构建"
        fi
    else
        log_warning "golangci-lint未安装，跳过代码检查"
        log_info "安装golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    fi
}

# 运行测试
run_tests() {
    log_info "运行单元测试..."
    if find . -name "*_test.go" | grep -q .; then
        go test ./... -v
        if [ $? -eq 0 ]; then
            log_success "测试通过"
        else
            log_error "测试失败"
            exit 1
        fi
    else
        log_info "未找到测试文件，跳过测试"
    fi
}

# 构建应用
build_app() {
    log_info "构建应用..."
    
    # 设置构建信息
    BUILD_TIME=$(date +"%Y-%m-%d %H:%M:%S")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    VERSION=${VERSION:-"dev"}
    
    # 构建参数
    LDFLAGS="-X 'main.BuildTime=$BUILD_TIME' -X 'main.GitCommit=$GIT_COMMIT' -X 'main.Version=$VERSION'"
    
    go build -ldflags "$LDFLAGS" -o box-manage-service main.go
    
    if [ $? -eq 0 ]; then
        log_success "构建完成: box-manage-service"
    else
        log_error "构建失败"
        exit 1
    fi
}

# 运行应用
run_app() {
    log_info "启动应用..."
    
    # 设置默认环境变量
    export DB_HOST=${DB_HOST:-"localhost"}
    export DB_PORT=${DB_PORT:-"5432"}
    export DB_USER=${DB_USER:-"postgres"}
    export DB_NAME=${DB_NAME:-"box_management"}
    export DB_SSLMODE=${DB_SSLMODE:-"disable"}
    export LISTEN_PORT=${LISTEN_PORT:-"8080"}
    
    log_info "数据库配置: $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME"
    log_info "服务端口: $LISTEN_PORT"
    
    ./box-manage-service
}

# Docker构建
build_docker() {
    log_info "构建Docker镜像..."
    
    IMAGE_NAME=${IMAGE_NAME:-"box-manage-service"}
    TAG=${TAG:-"latest"}
    
    docker build -t "$IMAGE_NAME:$TAG" .
    
    if [ $? -eq 0 ]; then
        log_success "Docker镜像构建完成: $IMAGE_NAME:$TAG"
    else
        log_error "Docker镜像构建失败"
        exit 1
    fi
}

# 清理构建产物
clean() {
    log_info "清理构建产物..."
    rm -f box-manage-service
    rm -rf docs/
    log_success "清理完成"
}

# 显示帮助信息
show_help() {
    echo "AI盒子管理系统后端服务构建脚本"
    echo ""
    echo "使用方法:"
    echo "  $0 [命令]"
    echo ""
    echo "命令:"
    echo "  check      检查开发环境"
    echo "  deps       安装依赖"
    echo "  docs       生成API文档"
    echo "  lint       代码检查"
    echo "  test       运行测试"
    echo "  build      构建应用"
    echo "  run        运行应用"
    echo "  dev        开发模式（构建并运行）"
    echo "  docker     构建Docker镜像"
    echo "  clean      清理构建产物"
    echo "  all        完整构建流程"
    echo "  help       显示帮助信息"
    echo ""
    echo "环境变量:"
    echo "  DB_HOST    数据库主机 (默认: localhost)"
    echo "  DB_PORT    数据库端口 (默认: 5432)"
    echo "  DB_USER    数据库用户 (默认: postgres)"
    echo "  DB_PASSWORD 数据库密码"
    echo "  DB_NAME    数据库名称 (默认: box_management)"
    echo "  LISTEN_PORT 服务端口 (默认: 8080)"
    echo ""
    echo "示例:"
    echo "  $0 dev                    # 开发模式"
    echo "  $0 build                  # 仅构建"
    echo "  DB_PASSWORD=123 $0 run    # 设置密码并运行"
}

# 主逻辑
main() {
    case "${1:-dev}" in
        "check")
            check_go_env
            ;;
        "deps")
            install_deps
            ;;
        "docs")
            generate_docs
            ;;
        "lint")
            lint_code
            ;;
        "test")
            run_tests
            ;;
        "build")
            check_go_env
            install_deps
            build_app
            ;;
        "run")
            run_app
            ;;
        "dev")
            check_go_env
            install_deps
            generate_docs
            build_app
            run_app
            ;;
        "docker")
            build_docker
            ;;
        "clean")
            clean
            ;;
        "all")
            check_go_env
            install_deps
            generate_docs
            lint_code
            run_tests
            build_app
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            log_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

# 捕获Ctrl+C信号
trap 'log_warning "构建被中断"; exit 130' INT

# 执行主逻辑
main "$@"



