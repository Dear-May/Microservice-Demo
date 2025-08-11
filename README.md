# 微服务架构演示项目

这是一个基于Docker Compose的微服务架构演示项目，包含用户认证、产品管理、服务发现和API网关等功能。

## 架构组件

### 1. 数据库服务
- **MySQL**: 存储用户和产品数据
- **LDAP**: 提供LDAP认证服务

### 2. 配置和服务发现
- **Consul**: 提供服务发现和配置管理功能

### 3. 微服务
- **UAA (User Authentication & Authorization)**: 用户认证服务
  - 数据库用户名密码登录
  - GitHub OAuth2登录
  - LDAP登录
- **Product**: 产品管理服务
  - 产品CRUD操作
  - 基于角色的权限控制
- **Gateway**: API网关
  - 反向代理
  - 路由转发
  - 统一入口点

## 角色权限设计

- **PRODUCT_ADMIN**: 拥有所有权限（EDITOR + USER）
- **EDITOR**: 拥有USER权限 + 产品编辑权限
- **USER**: 基础权限，只能查看产品

## 快速开始

### 1. 环境准备
确保已安装：
- Docker
- Docker Compose

### 2. 配置GitHub OAuth2（可选）
如果需要GitHub登录功能，请：
1. 在GitHub创建OAuth应用
2. 复制Client ID和Client Secret
3. 创建`.env`文件并填入配置：
```bash
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
```

### 3. 启动服务
```bash
docker-compose up -d
```

### 4. 访问服务
- **API网关**: http://localhost:7573
- **Consul UI**: http://localhost:8500
- **MySQL**: localhost:3306
- **LDAP**: localhost:389

## API接口

### 认证接口
- `POST /auth/login` - 数据库登录
- `GET /auth/github` - GitHub OAuth2登录
- `GET /auth/github/callback` - GitHub回调
- `POST /auth/register` - 用户注册
- `GET /auth/validate` - 验证令牌

### 产品接口
- `GET /products` - 获取产品列表（需要USER角色）
- `GET /products/:id` - 获取单个产品（需要USER角色）
- `POST /products` - 创建产品（需要EDITOR角色）
- `PUT /products/:id` - 更新产品（需要EDITOR角色）
- `DELETE /products/:id` - 删除产品（需要EDITOR角色）

## 默认用户

数据库初始化时会创建以下默认用户（密码都是：password123）：
- `admin` - PRODUCT_ADMIN角色
- `editor` - EDITOR角色
- `user` - USER角色

## 测试示例

### 1. 用户登录
```bash
curl -X POST http://localhost:7573/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password123"}'
```

### 2. 获取产品列表
```bash
curl -X GET http://localhost:7573/products \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 3. 创建产品
```bash
curl -X POST http://localhost:7573/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"name":"测试产品","description":"这是一个测试产品","price":99.99}'
```

## 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| Gateway | 7573 | API网关（对外端口） |
| Consul | 8500 | 服务发现和配置中心 |
| MySQL | 3306 | 数据库 |
| LDAP | 389 | LDAP服务 |

## 项目结构

```
Demo/
├── docker-compose.yaml    # Docker Compose配置
├── init.sql              # 数据库初始化脚本
├── env.example           # 环境变量示例
├── README.md             # 项目说明
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

## 故障排除

1. **服务启动失败**: 检查端口是否被占用
2. **数据库连接失败**: 等待MySQL完全启动后再启动其他服务
3. **GitHub登录失败**: 检查OAuth2配置是否正确
4. **权限验证失败**: 确认用户角色和令牌有效性

## 开发说明

- 所有Go服务都使用Gin框架
- 数据库使用GORM ORM
- 认证使用JWT令牌
- 服务发现使用Consul
- 容器化使用Docker 