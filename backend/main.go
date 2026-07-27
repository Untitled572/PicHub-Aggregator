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
	"github.com/pichub/backend/middleware"
	"github.com/pichub/backend/service"
	"github.com/pichub/backend/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5721"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/pichub.db"
	}
	os.MkdirAll("./data", 0755)
	os.MkdirAll("./cache", 0755)

	cfgPath := "config.json"
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		log.Println("config.json not found, using default config")
	} else {
		cfg, err := config.LoadConfig(cfgPath)
		if err != nil {
			log.Printf("warning: failed to load config: %v", err)
		} else {
			log.Printf("loaded config with %d preset sources", len(cfg.Sources))
		}
	}

	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}
	defer st.Close()

	checker := service.NewHealthChecker(st)
	checker.Start()
	defer checker.Stop()

	h := handler.NewHandler(st)
	r := gin.Default()

	r.Use(middleware.CORS())
	r.Use(middleware.RateLimit(st))

	r.GET("/health", h.HealthCheck)
	r.GET("/random", h.RandomImage)
	r.POST("/random/detect", h.DetectURL)
	r.POST("/api/sources/health-check", h.BatchHealthCheck)

	api := r.Group("/api")
	{
		api.GET("/sources", h.ListSources)
		api.GET("/sources/:id", h.GetSource)
		api.POST("/sources", h.CreateSource)
		api.PUT("/sources/:id", h.UpdateSource)
		api.DELETE("/sources/:id", h.DeleteSource)
		api.POST("/sources/:id/toggle", h.ToggleSource)
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", h.UpdateSettings)
		api.POST("/export", h.ExportRules)
		api.POST("/import", h.ImportRules)
	}

	distFS := embed.GetDistFS()
	if distFS != nil {
		r.GET("/", func(c *gin.Context) {
			data, err := fs.ReadFile(distFS, "index.html")
			if err != nil {
				c.String(500, "error")
				return
			}
			c.Data(200, "text/html; charset=utf-8", data)
		})
		r.StaticFS("/assets", http.FS(distFS))
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

	log.Printf("PicHub-Aggregator starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
