# Feishu Streaming Chat Bot with Dify API

一个基于飞书的流式对话机器人，支持Dify API，具有会话管理、流式响应等特性。

## 功能特点

- 流式对话：实时显示AI响应
- 会话管理：支持多用户会话隔离
- 内存优化：智能会话清理机制
- 配置灵活：支持多种AI提供商
- 部署简单：提供Docker部署方案

## 系统要求

- Ubuntu 24.04 LTS
- Docker & Docker Compose
- 8GB以上可用内存（服务使用6GB）
- 1GB以上可用磁盘空间

## 快速部署

### 1. 安装Docker和Docker Compose

```bash
# 更新系统
sudo apt update
sudo apt upgrade -y

# 安装必要工具
sudo apt install -y ca-certificates curl gnupg

# 添加Docker官方GPG密钥
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# 添加Docker仓库
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 将当前用户添加到docker组
sudo usermod -aG docker $USER
newgrp docker
```

### 2. 准备项目

```bash
# 创建项目目录
mkdir -p ~/feishu-bot && cd ~/feishu-bot

# 克隆代码
git clone <repository_url> .

# 创建日志目录
mkdir -p logs
```

### 3. 配置文件设置

```bash
# 复制配置文件
cp code/config.example.yaml code/config.yaml

# 编辑配置文件
vim code/config.yaml
```

需要修改的关键配置：
```yaml
# Feishu配置
APP_ID: "your_app_id"  # 飞书应用的App ID
APP_SECRET: "your_app_secret"  # 飞书应用的App Secret
APP_VERIFICATION_TOKEN: "your_token"  # 飞书的Verification Token
APP_ENCRYPT_KEY: "your_key"  # 飞书的Encrypt Key

# AI提供商配置
AI_API_URL: "your_dify_api_url"  # Dify的API地址
AI_API_KEY: "your_dify_api_key"  # Dify的API密钥
```

### 4. 构建和启动服务

```bash
# 构建并启动
docker compose up -d --build

# 查看日志
docker compose logs -f
```

### 5. 验证服务

```bash
# 检查服务状态
docker compose ps

# 测试服务是否正常
curl http://localhost:9000/ping

# 查看日志
tail -f logs/app.log
```

### 6. API接口说明

服务提供以下接口：

1. 健康检查接口：
   - 端点：`/ping`
   - 方法：GET
   - 返回：`{"message": "pong"}`
   - 用途：验证服务是否正常运行

2. 飞书事件订阅接口：
   - 端点：`/webhook/event`
   - 方法：POST
   - 用途：接收飞书消息和事件推送
   - 支持的消息类型：
     * 文本消息（text）
     * 图片消息（image）
     * 语音消息（audio）
   - 特性：
     * 支持多轮对话（通过sessionId关联）
     * 支持流式响应
     * 支持并发消息处理
     * 消息防重复处理
     * 超时保护（30秒）
     * 访问频率控制

3. 飞书卡片回调接口：
   - 端点：`/webhook/card`
   - 方法：POST
   - 用途：处理飞书消息卡片交互
   - 特性：
     * 支持卡片按钮交互
     * 支持卡片实时更新
     * 支持消息处理状态展示

在飞书开发者后台配置 Webhook 地址时：
- 事件订阅：`http(s)://your-domain:9000/webhook/event`
- 卡片回调：`http(s)://your-domain:9000/webhook/card`

测试方法：
```bash
# 测试URL验证（已验证可用）
curl -X POST http://your-domain:9000/webhook/event \
  -H "Content-Type: application/json" \
  -H "X-Lark-Request-Type: URL_VERIFICATION" \
  -d '{"challenge": "test123", "token": "test", "type": "url_verification"}'

# 实际返回
{"challenge":"test123"}
```

注意事项：
1. 确保配置了正确的 Verification Token 和 Encrypt Key
2. 建议在生产环境使用 HTTPS
3. 接口支持加密消息格式
4. 建议配置访问频率限制（ACCESS_CONTROL_MAX_COUNT_PER_USER_PER_DAY）
5. Challenge响应格式必须完全匹配，不能有多余的空格或其他字段

## 维护命令

### 服务管理

```bash
# 创建项目目录
mkdir -p ~/feishu-bot && cd ~/feishu-bot

# 克隆代码
git clone <repository_url> .

# 创建日志目录
mkdir -p logs
```

### 3. 配置文件设置

```bash
# 复制配置文件
cp code/config.example.yaml code/config.yaml

# 编辑配置文件
vim code/config.yaml
```

需要修改的关键配置：
```yaml
# Feishu配置
APP_ID: "your_app_id"  # 飞书应用的App ID
APP_SECRET: "your_app_secret"  # 飞书应用的App Secret
APP_VERIFICATION_TOKEN: "your_token"  # 飞书的Verification Token
APP_ENCRYPT_KEY: "your_key"  # 飞书的Encrypt Key

# AI提供商配置
AI_API_URL: "your_dify_api_url"  # Dify的API地址
AI_API_KEY: "your_dify_api_key"  # Dify的API密钥
```

### 4. 构建和启动服务

```bash
# 构建并启动
docker compose up -d --build

# 查看日志
docker compose logs -f
```

### 5. 验证服务

```bash
# 检查服务状态
docker compose ps

# 测试服务是否正常
curl http://localhost:9000/ping

# 查看日志
tail -f logs/app.log
```

### 6. API接口说明

服务提供以下接口：

1. 健康检查接口：
   - 端点：`/ping`
   - 方法：GET
   - 返回：`{"message": "pong"}`
   - 用途：验证服务是否正常运行

2. 飞书事件订阅接口：
   - 端点：`/webhook/event`
   - 方法：POST
   - 用途：接收飞书消息和事件推送
   - 支持的消息类型：
     * 文本消息（text）
     * 图片消息（image）
     * 语音消息（audio）
   - 特性：
     * 支持多轮对话（通过sessionId关联）
     * 支持流式响应
     * 支持并发消息处理
     * 消息防重复处理
     * 超时保护（30秒）
     * 访问频率控制

3. 飞书卡片回调接口：
   - 端点：`/webhook/card`
   - 方法：POST
   - 用途：处理飞书消息卡片交互
   - 特性：
     * 支持卡片按钮交互
     * 支持卡片实时更新
     * 支持消息处理状态展示

在飞书开发者后台配置 Webhook 地址时：
- 事件订阅：`http(s)://your-domain:9000/webhook/event`
- 卡片回调：`http(s)://your-domain:9000/webhook/card`

# Feishu Streaming Chat Bot with Dify API

一个基于飞书的流式对话机器人，支持Dify API，具有会话管理、流式响应等特性。

## 功能特点

- 流式对话：实时显示AI响应
- 会话管理：支持多用户会话隔离
- 内存优化：智能会话清理机制
- 配置灵活：支持多种AI提供商
- 部署简单：提供Docker部署方案

## 系统要求

- Ubuntu 24.04 LTS
- Docker & Docker Compose
- 8GB以上可用内存（服务使用6GB）
- 1GB以上可用磁盘空间

## 快速部署

### 1. 安装Docker和Docker Compose

```bash
# 更新系统
sudo apt update
sudo apt upgrade -y

# 安装必要工具
sudo apt install -y ca-certificates curl gnupg

# 添加Docker官方GPG密钥
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# 添加Docker仓库
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 将当前用户添加到docker组
sudo usermod -aG docker $USER
newgrp docker
```

### 2. 准备项目

```bash
# 创建项目目录
mkdir -p ~/feishu-bot && cd ~/feishu-bot

# 克隆代码
git clone <repository_url> .

# 创建日志目录
mkdir -p logs
```

### 3. 配置文件设置

```bash
# 复制配置文件
cp code/config.example.yaml code/config.yaml

# 编辑配置文件
vim code/config.yaml
```

需要修改的关键配置：
```yaml
# Feishu配置
APP_ID: "your_app_id"  # 飞书应用的App ID
APP_SECRET: "your_app_secret"  # 飞书应用的App Secret
APP_VERIFICATION_TOKEN: "your_token"  # 飞书的Verification Token
APP_ENCRYPT_KEY: "your_key"  # 飞书的Encrypt Key

# AI提供商配置
AI_API_URL: "your_dify_api_url"  # Dify的API地址
AI_API_KEY: "your_dify_api_key"  # Dify的API密钥
```

### 4. 构建和启动服务

```bash
# 构建并启动
docker compose up -d --build

# 查看日志
docker compose logs -f
```

### 5. 验证服务

```bash
# 检查服务状态
docker compose ps

# 测试服务是否正常
curl http://localhost:9000/ping

# 查看日志
tail -f logs/app.log
```

### 6. API接口说明

服务提供以下接口：

1. 健康检查接口：
   - 端点：`/ping`
   - 方法：GET
   - 返回：`{"message": "pong"}`
   - 用途：验证服务是否正常运行

2. 飞书事件订阅接口：
   - 端点：`/webhook/event`
   - 方法：POST
   - 用途：接收飞书消息和事件推送
   - 支持的消息类型：
     * 文本消息（text）
     * 图片消息（image）
     * 语音消息（audio）
   - 特性：
     * 支持多轮对话（通过sessionId关联）
     * 支持流式响应
     * 支持并发消息处理
     * 消息防重复处理
     * 超时保护（30秒）
     * 访问频率控制

3. 飞书卡片回调接口：
   - 端点：`/webhook/card`
   - 方法：POST
   - 用途：处理飞书消息卡片交互
   - 特性：
     * 支持卡片按钮交互
     * 支持卡片实时更新
     * 支持消息处理状态展示

在飞书开发者后台配置 Webhook 地址时：
- 事件订阅：`http(s)://your-domain:9000/webhook/event`
- 卡片回调：`http(s)://your-domain:9000/webhook/card`

测试方法：
```bash
# 测试URL验证（已验证可用）
curl -X POST https://feishubotstreaming.sdx.pub/webhook/event \
  -H "Content-Type: application/json" \
  -H "X-Lark-Request-Type: URL_VERIFICATION" \
  -d '{"challenge": "test123", "token": "test", "type": "url_verification"}'

# 实际返回
{"challenge":"test123"}
```

注意事项：
1. 确保配置了正确的 Verification Token 和 Encrypt Key
2. 建议在生产环境使用 HTTPS
3. 接口支持加密消息格式
4. 建议配置访问频率限制（ACCESS_CONTROL_MAX_COUNT_PER_USER_PER_DAY）
5. Challenge响应格式必须完全匹配，不能有多余的空格或其他字段

## 维护命令

### 服务管理

```bash
# 重启服务
docker compose restart

# 停止服务
docker compose down

# 更新服务
docker compose pull
docker compose up -d
```

### 日志查看

```bash
# 查看容器日志
docker compose logs -f

# 查看应用日志
tail -f logs/app.log
```

### 监控命令

```bash
# 查看资源使用
docker stats feishu-bot

# 检查配置
docker compose config
```

## 故障排查

### 1. 服务无法启动

检查以下几点：
- 配置文件是否正确
- 端口是否被占用
- 日志中是否有错误信息

### 2. 内存问题

如果遇到内存问题：
```bash
# 检查内存使用
docker stats

# 调整内存限制（docker-compose.yaml）
services:
  feishu-bot:
    deploy:
      resources:
        limits:
          memory: 6G
```

### 3. 连接问题

检查网络连接：
```bash
# 检查端口
netstat -tlnp | grep 9000

# 检查容器网络
docker network inspect bridge
```

## 性能优化

### 1. 内存优化

会话缓存配置（code/services/sessionCache.go）：
```go
const (
    MaxSessionsPerUser = 10
    MaxTotalSessions  = 10000
    MaxMessageLength  = 4096
    MemoryLimit      = 4 * 1024 * 1024 * 1024 // 4GB会话缓存限制（总内存6GB）
)
```

### 2. 日志优化

日志配置（docker-compose.yaml）：
```yaml
logging:
  driver: "json-file"
  options:
    max-size: "100m"
    max-file: "3"
```

## 安全建议

1. 使用HTTPS
2. 定期更新密钥
3. 启用访问控制
4. 监控异常访问
5. 定期备份配置

## 监控建议

创建监控脚本：
```bash
#!/bin/bash
# 检查服务状态
docker compose ps
# 检查内存使用
docker stats --no-stream feishu-bot
# 检查日志错误
tail -n 100 logs/app.log | grep -i error
```

## 常见问题

1. Q: 服务启动失败
   A: 检查配置文件和日志

2. Q: 内存使用过高
   A: 调整会话缓存参数

3. Q: 响应延迟高
   A: 检查网络和服务器负载

4. Q: 日志文件过大
   A: 检查日志轮转配置

## 注意事项

1. 内存使用说明：
   - 总内存限制：6GB（Docker配置）
   - 会话缓存限制：4GB
   - 系统和其他使用：2GB
2. 确保配置文件中的所有密钥和Token都已正确填写
2. 确保9000端口未被占用
3. 确保有足够的磁盘空间和内存
4. 建议配置HTTPS（生产环境）
5. 定期检查日志和监控

## 许可证

MIT License

## 贡献指南

欢迎提交Issue和Pull Request

## 联系方式

- 作者：[Your Name]
- 邮箱：[Your Email]
