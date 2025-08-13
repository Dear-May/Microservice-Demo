package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/consul/api"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Product struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
}

type UpdateProductRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
}

type UserClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

var (
	db           *gorm.DB
	consulClient *api.Client
	jwtSecret    []byte
)

func main() {
	systemToken := generateSystemToken()
	err := os.Setenv("SYSTEM_TOKEN", systemToken)
	if err != nil {
		return
	}
	log.Println("系统令牌:", os.Getenv("SYSTEM_TOKEN"))

	// 初始化JWT密钥
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))

	initDB()
	initConsul()
	registerService()

	r := gin.Default()

	// 中间件：认证和权限检查
	r.Use(authMiddleware())

	// 产品相关路由
	products := r.Group("/products")
	{
		products.GET("", requireRole("USER"), getProducts)
		products.GET("/:id", requireRole("USER"), getProduct)
		products.POST("", requireRole("EDITOR"), createProduct)
		products.PUT("/:id", requireRole("EDITOR"), updateProduct)
		products.DELETE("/:id", requireRole("EDITOR"), deleteProduct)
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Println("产品服务启动在端口 8081")
	r.Run(":8081")
}

// 添加系统令牌生成函数
func generateSystemToken() string {
	claims := jwt.MapClaims{
		"system": "health_checker",
		"exp":    time.Now().Add(10 * 365 * 24 * time.Hour).Unix(), // 10年有效期
		"role":   "SYSTEM",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)
	return tokenString
}

func initDB() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	db.AutoMigrate(&Product{})
}

func initConsul() {
	config := api.DefaultConfig()
	config.Address = fmt.Sprintf("%s:%s", os.Getenv("CONSUL_HOST"), os.Getenv("CONSUL_PORT"))

	var err error
	consulClient, err = api.NewClient(config)
	if err != nil {
		log.Fatal("Consul客户端初始化失败:", err)
	}
}

func registerService() {
	registration := &api.AgentServiceRegistration{
		ID:      "product",
		Name:    "product",
		Port:    8081,
		Address: "product",
		Check: &api.AgentServiceCheck{
			HTTP: "http://product:8081/health",
			Header: map[string][]string{
				"Authorization": {"Bearer " + strings.TrimSpace(os.Getenv("SYSTEM_TOKEN"))}},
			Interval:                       "10s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "30s",
		},
	}

	err := consulClient.Agent().ServiceRegister(registration)
	if err != nil {
		log.Printf("服务注册失败: %v", err)
	}
}

// 认证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 豁免健康检查路由
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(401, gin.H{"error": "缺少认证令牌"})
			c.Abort()
			return
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "无效的令牌"})
			c.Abort()
			return
		}

		claims := token.Claims.(*UserClaims)
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// 角色权限检查中间件
func requireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetString("role")

		// 检查角色权限
		if !hasPermission(userRole, requiredRole) {
			c.JSON(403, gin.H{"error": "权限不足"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// 检查用户是否有指定角色的权限
func hasPermission(userRole, requiredRole string) bool {
	switch userRole {
	case "PRODUCT_ADMIN":
		return true // 拥有所有权限
	case "EDITOR":
		return requiredRole == "USER" || requiredRole == "EDITOR"
	case "USER":
		return requiredRole == "USER"
	default:
		return false
	}
}

// 获取产品列表
func getProducts(c *gin.Context) {
	var products []Product
	if err := db.Find(&products).Error; err != nil {
		c.JSON(500, gin.H{"error": "获取产品列表失败"})
		return
	}

	c.JSON(200, products)
}

// 获取单个产品
func getProduct(c *gin.Context) {
	id := c.Param("id")
	productID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的产品ID"})
		return
	}

	var product Product
	if err := db.First(&product, productID).Error; err != nil {
		c.JSON(404, gin.H{"error": "产品不存在"})
		return
	}

	c.JSON(200, product)
}

// 创建产品
func createProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求参数"})
		return
	}

	product := Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
	}

	if err := db.Create(&product).Error; err != nil {
		c.JSON(500, gin.H{"error": "创建产品失败"})
		return
	}

	c.JSON(201, product)
}

// 更新产品
func updateProduct(c *gin.Context) {
	id := c.Param("id")
	productID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的产品ID"})
		return
	}

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求参数"})
		return
	}

	var product Product
	if err := db.First(&product, productID).Error; err != nil {
		c.JSON(404, gin.H{"error": "产品不存在"})
		return
	}

	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Description != "" {
		product.Description = req.Description
	}
	if req.Price > 0 {
		product.Price = req.Price
	}

	if err := db.Save(&product).Error; err != nil {
		c.JSON(500, gin.H{"error": "更新产品失败"})
		return
	}

	c.JSON(200, product)
}

// 删除产品
func deleteProduct(c *gin.Context) {
	id := c.Param("id")
	productID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "无效的产品ID"})
		return
	}

	var product Product
	if err := db.First(&product, productID).Error; err != nil {
		c.JSON(404, gin.H{"error": "产品不存在"})
		return
	}

	if err := db.Delete(&product).Error; err != nil {
		c.JSON(500, gin.H{"error": "删除产品失败"})
		return
	}

	c.JSON(200, gin.H{"message": "产品删除成功"})
}
