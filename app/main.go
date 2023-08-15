package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"ldap-http-service/config"
	"ldap-http-service/lib/logger"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 加载配置
	config.LoadConfig()
	// 获取Logger
	LOGGER, err := logger.NewLogger("gin")

	// 启动gin并配置中间件等
	router := gin.New()
	router.Use(processRequest(LOGGER), ginLog(logger.GinLogger))
	gin.DefaultWriter = logger.GinLogger.Writer()
	// 添加Gin的恢复中间件，以便在出现panic时恢复运行并记录错误
	router.Use(gin.Recovery())

	err = router.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		return
	}

	// 注册路由
	router.GET("/ldap/healthz", handleHealthz)
	router.GET("/ldap/availability", handleCheckAvailability)
	router.GET("/ldap/user/:user_id", handleGetUser)
	router.POST("/ldap/user", handleNewEnableUser)
	router.PATCH("/ldap/user/:user_id", handleUserUpdate)
	router.POST("/ldap/user/:user_id/password", handleUserPwd)
	router.GET("/ldap/group/:group_id", handleGetGroup)
	router.POST("/ldap/group", handleNewGroup)
	router.PATCH("/ldap/group/:group_id", handleGroupUpdate)
	router.PUT("/ldap/group/:group_id/member", handleGroupMember)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.GinConfig.Listen, config.GinConfig.Port),
		Handler: router,
	}

	go func() {
		// service connections
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
