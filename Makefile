.PHONY: proto proto-clean help run run-restart build

# 默认目标
.DEFAULT_GOAL := help

# Proto 文件目录
PROTO_DIR := grpc/proto
# 生成的 Go 文件输出根目录
PROTO_GEN_DIR := grpc/proto/gen

# 编译所有 proto 文件
proto:
	@echo "正在编译 proto 文件..."
	@mkdir -p $(PROTO_GEN_DIR)
	@for proto_file in $(PROTO_DIR)/*.proto; do \
		package_name=$$(grep -E '^package ' $$proto_file | awk '{print $$2}' | sed 's/;//'); \
		output_dir=$(PROTO_GEN_DIR)/$$package_name; \
		echo "编译 $$proto_file (package: $$package_name) 到 $$output_dir..."; \
		mkdir -p $$output_dir; \
		protoc --go_out=$$output_dir \
			--go_opt=paths=source_relative \
			--go-grpc_out=$$output_dir \
			--go-grpc_opt=paths=source_relative \
			-I=$(PROTO_DIR) \
			$$proto_file; \
	done
	@echo "Proto 文件编译完成！"

# 清理生成的 proto 文件
proto-clean:
	@echo "清理生成的 proto 文件..."
	@rm -rf $(PROTO_GEN_DIR)
	@echo "清理完成！"

# 构建项目
build: proto
	@echo "构建项目..."
	@go build -o bin/newer_helper .
	@echo "构建完成！"

# 运行项目（自动编译 proto）
run: proto
	@echo "运行项目..."
	@go run .

# 运行项目并在异常退出时自动重启
run-restart: proto
	@echo "运行项目（带自动重启）..."
	@echo "按 Ctrl+C 可以完全停止"
	@restart_count=0; \
	while true; do \
		echo ""; \
		echo "========================================"; \
		if [ $$restart_count -eq 0 ]; then \
			echo "启动程序..."; \
		else \
			echo "第 $$restart_count 次重启..."; \
		fi; \
		echo "========================================"; \
		start_time=$$(date +%s); \
		go run .; \
		exit_code=$$?; \
		end_time=$$(date +%s); \
		runtime=$$((end_time - start_time)); \
		echo ""; \
		echo "程序退出，退出码: $$exit_code"; \
		if [ $$exit_code -eq 0 ]; then \
			echo "程序正常退出，不再重启"; \
			break; \
		fi; \
		restart_count=$$((restart_count + 1)); \
		if [ $$runtime -lt 5 ]; then \
			echo "程序运行时间过短（$$runtime 秒），等待 5 秒后重启..."; \
			sleep 5; \
		else \
			echo "等待 2 秒后重启..."; \
			sleep 2; \
		fi; \
	done

# 显示帮助信息
help:
	@echo "可用的 make 命令："
	@echo "  make proto        - 编译所有 proto 文件"
	@echo "  make proto-clean  - 清理生成的 proto 文件"
	@echo "  make build        - 编译 proto 文件并构建项目"
	@echo "  make run          - 编译 proto 文件并运行项目"
	@echo "  make run-restart  - 运行项目并在异常退出时自动重启"
	@echo "  make help         - 显示此帮助信息"
