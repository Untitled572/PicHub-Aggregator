// @title PicHub Aggregator API
// @version 0.6.5
// @description PicHub-Aggregator 图片聚合分发服务。管理接口支持会话和 Bearer token 鉴权，图片分发接口公开。
// @termsOfService https://github.com/untitled572/pichub-aggregator
// @contact.name PicHub
// @license.name MIT
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/pichub/backend/config"
	_ "github.com/pichub/backend/docs"
	"github.com/pichub/backend/embed"
	"github.com/pichub/backend/handler"
	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/middleware"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/monitor"
	"github.com/pichub/backend/service"
	"github.com/pichub/backend/store"
)

//go:generate swag init -g main.go --parseDependency --parseInternal -o docs

func main() {
	gin.SetMode(gin.ReleaseMode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5721"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/pichub.db"
	}
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Printf("warning: failed to create data dir: %v", err)
	}
	if err := os.MkdirAll("./cache", 0755); err != nil {
		log.Printf("warning: failed to create cache dir: %v", err)
	}
	if err := os.MkdirAll("./data/images", 0755); err != nil {
		log.Printf("warning: failed to create images dir: %v", err)
	}

	logger.Init("./data")
	defer logger.Close()

	logger.System("PicHub-Aggregator initializing")

	cfgPath := "config.json"
	var cfg *config.AppConfig
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		logger.System("config.json not found, using default config")
	} else {
		cfg, err = config.LoadConfig(cfgPath)
		if err != nil {
			logger.Error("failed to load config: %v", err)
		} else {
			logger.System("loaded config with %d preset sources", len(cfg.Sources))
		}
	}

	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("failed to flush and close store: %v", err)
		}
	}()

	// seed config.json sources into DB (if not already seeded)
	if cfg != nil {
		var srcModels []model.Source
		for _, sc := range cfg.Sources {
			srcModels = append(srcModels, model.Source{
				Name:        sc.Name,
				URL:         sc.URL,
				RespType:    sc.RespType,
				JsonPath:    sc.JsonPath,
				Weight:      sc.Weight,
				Categories:  sc.Categories,
				Enabled:     true,
				Status:      "normal",
				SuccessRate: 100.0,
			})
		}
		if err := st.SeedSources(srcModels); err != nil {
			logger.Error("failed to seed sources: %v", err)
		}
	}

	proxyConfig := service.NewProxyConfig()
	settings, _ := st.GetSettings()
	if settings != nil {
		proxyConfig.Update(settings.ProxyEnabled, settings.ProxyURL)
	}

	checker := service.NewHealthChecker(st, proxyConfig)
	checker.Start()
	defer checker.Stop()

	proxyCache := service.NewProxyCache(st, "./cache")
	imageStore := service.NewImageStore(st, "./data/images", proxyConfig)
	imageStore.CleanupOrphanPooled()

	// 启动时清理无效图片（宽度/高度 < 1、格式未知）
	if fileIDs, count, err := st.CleanupInvalidImages(); err == nil && count > 0 {
		logger.System("cleaned %d invalid cached images", count)
		for _, fid := range fileIDs {
			pattern := "./data/images/*/" + fid + ".*"
			if matches, err := filepath.Glob(pattern); err == nil {
				for _, m := range matches {
					os.Remove(m)
				}
			}
		}
	}
	sourceMonitor := monitor.NewSourceMonitor(st)
	engine := service.NewEngine(st, proxyCache, imageStore, proxyConfig, sourceMonitor)
	h := handler.NewHandlerWithImageStore(st, engine, checker, imageStore, proxyConfig)
	r := gin.New()
	r.SetTrustedProxies(strings.Split(os.Getenv("TRUSTED_PROXIES"), ","))
	r.Use(middleware.PanicRecover())
	r.Use(middleware.RequestID())
	r.Use(middleware.AccessLog())
	r.Use(middleware.CORS())
	r.Use(middleware.ServerTime())

	rateLimitMW := middleware.RateLimit(st)

	// Swagger API 文档: SWAGGER_ENABLED=false 时关闭
	if os.Getenv("SWAGGER_ENABLED") != "false" {
		swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler)
		r.GET("/swagger/*any", func(c *gin.Context) {
			// gin-swagger 只匹配已知资源文件名, /swagger/ 根路径会 404, 重定向到 index.html
			if p := strings.TrimSuffix(c.Param("any"), "/"); p == "" || p == "swagger" {
				c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
				return
			}
			swaggerHandler(c)
		})
	}

	r.GET("/ping", h.HealthCheck)
	r.GET("/random", rateLimitMW, h.RandomImage)
	r.GET("/e/:name", rateLimitMW, h.EndpointImage)
	r.GET("/images/:file_id", h.ServeImage)
	registerManagementRoutes(r, h, st)

	distFS := embed.GetDistFS()
	assetsFS := embed.GetAssetsFS()
	if distFS != nil {
		r.GET("/", func(c *gin.Context) {
			data, err := fs.ReadFile(distFS, "index.html")
			if err != nil {
				c.String(500, "error")
				return
			}
			c.Data(200, "text/html; charset=utf-8", data)
		})
		if assetsFS != nil {
			r.StaticFS("/assets", http.FS(assetsFS))
		}
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/random/") || strings.HasPrefix(c.Request.URL.Path, "/e/") || strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			data, err := fs.ReadFile(distFS, "index.html")
			if err != nil {
				c.String(500, "error")
				return
			}
			c.Data(200, "text/html; charset=utf-8", data)
		})
	}

	logger.System("PicHub-Aggregator starting on :%s", port)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server stopped: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP shutdown: %v", err)
			_ = server.Close()
		}
	}

}

// Keep management routes together so new endpoints inherit authentication.
func registerManagementRoutes(r *gin.Engine, h *handler.Handler, st *store.Store) {
	r.POST("/random/detect", middleware.AdminAuth(st), h.DetectURL)

	api := r.Group("/api")
	{
		api.POST("/login", h.Login)
		api.GET("/auth/check", h.CheckAuth)
		api.Use(middleware.AdminAuth(st))
		api.POST("/logout", h.Logout)
		api.POST("/sources/health-check", h.BatchHealthCheck)
		api.GET("/sources", h.ListSources)
		api.GET("/sources/:id", h.GetSource)
		api.POST("/sources", h.CreateSource)
		api.PUT("/sources/:id", h.UpdateSource)
		api.DELETE("/sources/:id", h.DeleteSource)
		api.POST("/sources/:id/toggle", h.ToggleSource)
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", h.UpdateSettings)
		api.GET("/tags", h.GetTags)
		api.PUT("/tags", h.UpdateTags)
		api.GET("/endpoints", h.ListEndpoints)
		api.POST("/endpoints", h.CreateEndpoint)
		api.PUT("/endpoints/:id", h.UpdateEndpoint)
		api.DELETE("/endpoints/:id", h.DeleteEndpoint)
		api.POST("/endpoints/:id/toggle", h.ToggleEndpoint)
		api.GET("/health", h.GetHealthStatus)
		api.GET("/stats", h.GetStats)
		api.GET("/stats/history", h.GetImageHistory)
		api.GET("/images/saved", h.ListSavedImages)
		api.POST("/images/:id/save", h.SaveImage)
		api.POST("/images/:id/like", h.LikeImage)
		api.POST("/images/:id/dislike", h.DislikeImage)

		api.DELETE("/images/:id/save", h.UnsaveImage)
		api.GET("/export", h.ExportData)
		api.POST("/export", h.ExportRules)
		api.POST("/import", h.ImportData)

	}
}
