package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/consul/api"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username" gorm:"unique;not null"`
	Password  string    `json:"-" gorm:"not null"`
	Email     string    `json:"email" gorm:"unique;not null"`
	Role      string    `json:"role" gorm:"default:'USER'"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type TokenResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type GitHubUser struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

var (
	db           *gorm.DB
	consulClient *api.Client
	jwtSecret    []byte
	githubOAuth  *oauth2.Config
)

func main() {
	// 初始化JWT密钥
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))

	initDB()
	initConsul()
	initGitHubOAuth()
	registerService()

	r := gin.Default()

	auth := r.Group("/auth")
	{
		auth.POST("/login", login)
		auth.GET("/github", githubLogin)
		auth.GET("/github/callback", githubCallback)
		auth.POST("/register", register)
		auth.GET("/validate", validateToken)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Println("UAA服务启动在端口 8080")
	r.Run(":8080")
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

	db.AutoMigrate(&User{})
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

func initGitHubOAuth() {
	githubOAuth = &oauth2.Config{
		ClientID:     os.Getenv("Ov23liEFail3pbo8vsw9"),
		ClientSecret: os.Getenv("da7cd39d998917c284c970c411193c3bf85e0703"),
		RedirectURL:  "http://localhost:7573/auth/github/callback",
		Scopes:       []string{"user:email"},
		Endpoint:     github.Endpoint,
	}
}

func registerService() {
	registration := &api.AgentServiceRegistration{
		ID:      "uaa",
		Name:    "uaa",
		Port:    8080,
		Address: "uaa",
		Check: &api.AgentServiceCheck{
			HTTP:                           "http://uaa:8080/health",
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

func login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求参数"})
		return
	}

	var user User
	if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(401, gin.H{"error": "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(401, gin.H{"error": "用户名或密码错误"})
		return
	}

	token := generateToken(user)
	c.JSON(200, TokenResponse{Token: token, User: user})
}

func githubLogin(c *gin.Context) {
	url := githubOAuth.AuthCodeURL("state")
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func githubCallback(c *gin.Context) {
	code := c.Query("code")

	token, err := githubOAuth.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(500, gin.H{"error": "GitHub认证失败"})
		return
	}

	client := githubOAuth.Client(context.Background(), token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		c.JSON(500, gin.H{"error": "获取GitHub用户信息失败"})
		return
	}
	defer resp.Body.Close()

	var githubUser GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		c.JSON(500, gin.H{"error": "解析GitHub用户信息失败"})
		return
	}

	var user User
	db.Where("username = ?", githubUser.Login).First(&user)
	if user.ID == 0 {
		user = User{
			Username: githubUser.Login,
			Email:    githubUser.Email,
			Role:     "USER",
		}
		db.Create(&user)
	}

	tokenString := generateToken(user)
	c.JSON(200, TokenResponse{Token: tokenString, User: user})
}

func register(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(400, gin.H{"error": "无效的请求参数"})
		return
	}

	var existingUser User
	if err := db.Where("username = ?", user.Username).First(&existingUser).Error; err == nil {
		c.JSON(400, gin.H{"error": "用户名已存在"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "密码加密失败"})
		return
	}

	user.Password = string(hashedPassword)
	if user.Role == "" {
		user.Role = "USER"
	}

	db.Create(&user)
	user.Password = ""
	c.JSON(201, user)
}

func validateToken(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		c.JSON(401, gin.H{"error": "缺少认证令牌"})
		return
	}

	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		c.JSON(401, gin.H{"error": "无效的令牌"})
		return
	}

	claims := token.Claims.(jwt.MapClaims)
	userID := uint(claims["user_id"].(float64))

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(401, gin.H{"error": "用户不存在"})
		return
	}

	user.Password = ""
	c.JSON(200, user)
}

func generateToken(user User) string {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)
	return tokenString
}
