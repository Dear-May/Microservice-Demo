# 微服务架构快速启动指南

## 项目概述

这是一个完整的微服务架构演示项目，包含：

- **UAA服务**: 用户认证和授权（数据库登录、GitHub OAuth2、LDAP登录）
- **产品服务**: 产品管理（CRUD操作，基于角色的权限控制）
- **API网关**: 统一入口，反向代理，路由转发
- **MySQL**: 数据存储
- **LDAP**: 目录服务
- **Consul**: 服务发现和配置管理

## 快速启动

### 前提条件

1. **安装Docker和Docker Compose**
2. **安装Go 1.21+** (从 https://golang.org/dl/ 下载)

### 启动步骤

1. **启动基础服务**
   ```bash
   docker-compose up -d mysql consul
   ```

2. **等待服务启动** (约15秒)
   ```bash
   docker-compose ps
   ```

3. **启动Go微服务** (在3个不同的终端中)
   ```bash
   # 终端1: UAA服务
   cd uaa
   go mod download
   go run main.go
   
   # 终端2: 产品服务
   cd product
   go mod download
   go run main.go
   
   # 终端3: 网关服务
   cd gateway
   go mod download
   go run main.go
   ```

## 验证服务

### 1. 检查服务状态
- Consul UI: http://localhost:8500
- API网关: http://localhost:7573

### 2. 测试API

**用户登录**:
```bash
curl -X POST http://localhost:7573/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password123"}'
```

**获取产品列表**:
```bash
curl -X GET http://localhost:7573/products \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**创建产品**:
```bash
curl -X POST http://localhost:7573/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"name":"测试产品","description":"这是一个测试产品","price":99.99}'
```

## 默认用户

| 用户名 | 密码 | 角色 | 权限 |
|--------|------|------|------|
| admin | password123 | PRODUCT_ADMIN | 所有权限 |
| editor | password123 | EDITOR | 产品编辑权限 |
| user | password123 | USER | 查看权限 |

## 角色权限

- **PRODUCT_ADMIN**: 拥有所有权限
- **EDITOR**: 拥有USER权限 + 产品编辑权限
- **USER**: 只能查看产品

## API接口

### 认证接口
- `POST /auth/login` - 数据库登录
- `GET /auth/github` - GitHub OAuth2登录
- `POST /auth/register` - 用户注册
- `GET /auth/validate` - 验证令牌

### 产品接口
- `GET /products` - 获取产品列表（需要USER角色）
- `GET /products/:id` - 获取单个产品（需要USER角色）
- `POST /products` - 创建产品（需要EDITOR角色）
- `PUT /products/:id` - 更新产品（需要EDITOR角色）
- `DELETE /products/:id` - 删除产品（需要EDITOR角色）

## 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| Gateway | 7573 | API网关（对外端口） |
| Consul | 8500 | 服务发现和配置中心 |
| MySQL | 3306 | 数据库 |
| UAA | 8080 | 用户认证服务 |
| Product | 8081 | 产品服务 |

## 故障排除

如果遇到问题，请查看 `TROUBLESHOOTING.md` 文件。

常见问题：
1. **Go命令未找到**: 安装Go环境
2. **端口被占用**: 修改docker-compose.yaml中的端口
3. **数据库连接失败**: 等待MySQL完全启动
4. **Docker镜像拉取失败**: 配置镜像加速器

## 项目结构

```
Demo/
├── docker-compose.yaml    # Docker Compose配置
├── init.sql              # 数据库初始化脚本
├── README.md             # 项目说明
├── QUICK_START.md        # 快速启动指南
├── TROUBLESHOOTING.md    # 故障排除指南
├── start-services.ps1    # PowerShell启动脚本
├── uaa/                  # 用户认证服务
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── product/              # 产品服务
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── gateway/              # API网关
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
└── ldap/                 # LDAP配置
    └── ldif/
        └── users.ldif
```

## 技术栈

- **后端**: Go + Gin + GORM
- **数据库**: MySQL
- **认证**: JWT + OAuth2
- **服务发现**: Consul
- **容器化**: Docker + Docker Compose
- **目录服务**: OpenLDAP

