package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pichub/backend/config"
	"github.com/pichub/backend/embed"
	"github.com/pichub/backend/handler"
	"github.com/pichub/backend/logger"
	"github.com/pichub/backend/middleware"
	"github.com/pichub/backend/model"
	"github.com/pichub/backend/service"
	"github.com/pichub/backend/store"
)

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
	defer st.Close()

	// seed config.json sources into DB (if not already seeded)
	if cfg != nil {
		var srcModels []model.Source
		for _, sc := range cfg.Sources {
			srcModels = append(srcModels, model.Source{
				Name:       sc.Name,
				URL:        sc.URL,
				RespType:   sc.RespType,
				JsonPath:   sc.JsonPath,
				Weight:     sc.Weight,
				Categories: sc.Categories,
				Enabled:    true,
				Status:     "normal",
				SuccessRate: 100.0,
			})
		}
		if err := st.SeedSources(srcModels); err != nil {
			logger.Error("failed to seed sources: %v", err)
		}
	}

	checker := service.NewHealthChecker(st)
	checker.Start()
	defer checker.Stop()

	proxyCache := service.NewProxyCache(st, "./cache")
	engine := service.NewEngine(st, proxyCache)
	h := handler.NewHandlerWithEngine(st, engine, checker)
	r := gin.New()
	r.SetTrustedProxies(strings.Split(os.Getenv("TRUSTED_PROXIES"), ","))
	r.Use(gin.Recovery())
	r.Use(middleware.AccessLog())
	r.Use(middleware.CORS())

	r.GET("/ping", h.HealthCheck)
	r.GET("/random", middleware.RateLimit(st), h.RandomImage)
	r.POST("/random/detect", h.DetectURL)
	r.POST("/api/sources/health-check", h.BatchHealthCheck)


	api := r.Group("/api")
	{
		api.GET("/sources", h.ListSources)
		api.GET("/sources/:id", h.GetSource)
		api.POST("/sources", middleware.AdminAuth(st), h.CreateSource)
		api.PUT("/sources/:id", middleware.AdminAuth(st), h.UpdateSource)
		api.DELETE("/sources/:id", middleware.AdminAuth(st), h.DeleteSource)
		api.POST("/sources/:id/toggle", middleware.AdminAuth(st), h.ToggleSource)
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", middleware.AdminAuth(st), h.UpdateSettings)
		api.GET("/tags", h.GetTags)
		api.PUT("/tags", middleware.AdminAuth(st), h.UpdateTags)
		api.GET("/health", h.GetHealthStatus)
		api.GET("/stats", h.GetStats)
		api.GET("/stats/history", h.GetImageHistory)
		api.POST("/export", middleware.AdminAuth(st), h.ExportRules)
		api.POST("/import", middleware.AdminAuth(st), h.ImportRules)
	}

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
			if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/random/") {
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
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
