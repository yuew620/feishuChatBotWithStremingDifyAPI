package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/larksuite/oapi-sdk-go/v3/card"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	sdkginext "github.com/larksuite/oapi-sdk-gin"
	"github.com/spf13/pflag"
	"gopkg.in/natefinch/lumberjack.v2"

	"start-feishubot/handlers"
	"start-feishubot/initialization"
	"start-feishubot/services"
	"start-feishubot/utils"
)

func main() {
	// 初始化角色列表
	initialization.InitRoleList()
	pflag.Parse()
	globalConfig := initialization.GetConfig()

	// 打印配置
	globalConfigPrettyString, _ := json.MarshalIndent(globalConfig, "", "    ")
	log.Println(string(globalConfigPrettyString))

	// 初始化日志
	var logger *lumberjack.Logger
	config := initialization.GetConfig()
	if config.EnableLog {
		logger = enableLog()
		defer utils.CloseLogger(logger)
	}

	// 初始化飞书客户端
	initialization.LoadLarkClient(*initialization.GetConfig())
	
	// 初始化卡片池
	initCardPool(context.Background())
	
	// 初始化handlers
	if err := handlers.InitHandlers(*initialization.GetConfig()); err != nil {
		log.Fatalf("failed to initialize handlers: %v", err)
	}

	// 创建事件处理器
	eventHandler := dispatcher.NewEventDispatcher(
		config.FeishuAppVerificationToken, config.FeishuAppEncryptKey).
		OnP2MessageReceiveV1(handlers.Handler).
		OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
			return handlers.ReadHandler(ctx, event)
		})

	// 创建卡片处理器
	cardHandler := larkcard.NewCardActionHandler(
		config.FeishuAppVerificationToken, config.FeishuAppEncryptKey,
		handlers.CardHandler())

	// 设置路由
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.POST("/webhook/event", func(c *gin.Context) {
		// Log all headers
		log.Printf("Request Headers:")
		for key, values := range c.Request.Header {
			log.Printf("%s: %v", key, values)
		}

		// Read and log request body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			return
		}
		log.Printf("Request Body: %s", string(body))
		
		// Restore body for later use
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		
		reqType := c.Request.Header.Get("X-Lark-Request-Type")
		log.Printf("Received webhook request with type: %s", reqType)
		
		// Handle URL verification first
		if handlers.HandleUrlVerification(c) {
			log.Printf("Finished handling URL verification request")
			return
		}
		
		// Handle other events
		log.Printf("Handling non-verification event")
		sdkginext.NewEventHandlerFunc(eventHandler)(c)
	})
	r.POST("/webhook/card",
		sdkginext.NewCardActionHandlerFunc(
			cardHandler))

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.HttpPort),
		Handler: r,
	}

	// 优雅关闭
	go func() {
		// 等待中断信号
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down server...")

		// 创建一个5秒超时的context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 关闭AI提供商
		if err := initialization.ShutdownAIProvider(); err != nil {
			log.Printf("Error shutting down AI provider: %v", err)
		}

		// 关闭卡片池
		initialization.ShutdownCardPool()

		// 关闭HTTP服务器
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}

		log.Println("Server exiting")
	}()

	// 启动服务器
	var err error
	if config.UseHttps {
		err = srv.ListenAndServeTLS(config.GetCertFile(), config.GetKeyFile())
	} else {
		err = srv.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initCardPool(ctx context.Context) {
	// 创建卡片创建函数
	createCardFn := func(ctx context.Context) (string, error) {
		card, err := handlers.CreateCardEntity(ctx, "正在处理中...")
		if err != nil {
			return "", fmt.Errorf("failed to create card: %w", err)
		}
		return card, nil
	}

	// 初始化卡片池
	if err := initialization.InitCardPool(createCardFn); err != nil {
		log.Printf("Warning: Failed to initialize card pool: %v", err)
	}
}

func enableLog() *lumberjack.Logger {
	logger := &lumberjack.Logger{
		Filename: "logs/app.log",
		MaxSize:  100,      // megabytes
		MaxAge:   365 * 10, // days
	}

	log.SetOutput(io.MultiWriter(logger, os.Stdout))
	log.SetFlags(log.Ldate | log.Ltime)
	log.Println("Starting application...")

	return logger
}
