# 项目名称
APP_NAME := myapp

# 构建目录
BUILD_DIR := bin
# 源码目录
SRC_DIR := cmd/app

# 默认目标
all: build

# 构建应用
build:
	@echo "正在构建应用..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(SRC_DIR)/main.go

# 运行应用（开发模式）
run:
	@echo "正在运行应用..."
	@go run $(SRC_DIR)/main.go

# 清理构建文件
clean:
	@echo "正在清理..."
	@rm -rf $(BUILD_DIR)/*
	@echo "清理完成"

# 安装依赖
deps:
	@echo "正在安装依赖..."
	@go mod tidy

# 显示帮助
help:
	@echo "可用命令:"
	@echo "  make         - 构建应用"
	@echo "  make run     - 运行应用（开发模式）"
	@echo "  make clean   - 清理构建文件"
	@echo "  make deps    - 安装依赖"
	@echo "  make help    - 显示帮助信息"