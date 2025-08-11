package main

import (
	"fmt"
	"log"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/consul/api"
)

var consulClient *api.Client

func main() {
	initConsul()

	r := gin.Default()

	// 认证相关路由 - 转发到UAA服务
	auth := r.Group("/auth")
	{
		auth.Any("/*path", proxyToService("uaa"))
	}

	// 产品相关路由 - 转发到产品服务
	products := r.Group("/products")
	{
		products.Any("/*path", proxyToService("product"))
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Println("API网关启动在端口 7573")
	r.Run(":7573")
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

// 代理到指定服务
func proxyToService(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Consul获取服务地址
		serviceAddr, err := getServiceAddress(serviceName)
		if err != nil {
			c.JSON(503, gin.H{"error": fmt.Sprintf("服务 %s 不可用", serviceName)})
			return
		}

		// 创建反向代理
		target, err := url.Parse(fmt.Sprintf("http://%s", serviceAddr))
		if err != nil {
			c.JSON(500, gin.H{"error": "代理配置错误"})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		// 修改请求路径 - 移除服务名前缀
		originalPath := c.Request.URL.Path
		if serviceName == "uaa" {
			c.Request.URL.Path = strings.TrimPrefix(originalPath, "/auth")
		} else if serviceName == "product" {
			c.Request.URL.Path = strings.TrimPrefix(originalPath, "/products")
		}
		if c.Request.URL.Path == "" {
			c.Request.URL.Path = "/"
		}

		// 转发请求
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// 从Consul获取服务地址
func getServiceAddress(serviceName string) (string, error) {
	services, _, err := consulClient.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return "", err
	}

	if len(services) == 0 {
		return "", fmt.Errorf("服务 %s 未找到", serviceName)
	}

	service := services[0]
	return fmt.Sprintf("%s:%d", service.Service.Address, service.Service.Port), nil
}
