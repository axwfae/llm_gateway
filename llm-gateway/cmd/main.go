package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"llm_gateway/internal/api"
	"llm_gateway/internal/proxy"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/version"
	"llm_gateway/internal/webui"

	"github.com/gin-gonic/gin"
)

var store *storage.Storage

func main() {
	rand.Seed(time.Now().UnixNano())

	configPath := flag.String("config", "config/config.yaml", "Path to config file")
	apiPort := flag.Int("api-port", 18869, "API proxy port")
	uiPort := flag.Int("ui-port", 18866, "Web UI port")
	debugMode := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	if *debugMode || os.Getenv("DEBUG") == "true" {
		*debugMode = true
		gin.SetMode(gin.DebugMode)
	}

	log.Printf("LLM Gateway v%s (build: %s)", version.Version, version.BuildDate)
	log.Printf("Debug mode: %v", *debugMode)

	var err error
	store, err = storage.NewStorage(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	proxyHandler := &proxy.Handler{Store: store, Debug: *debugMode}
	apiHandler := &api.Handler{Store: store, Debug: *debugMode}

	settings := store.GetSettings()
	if settings.EnableAutoCheckAPIKey {
		go runAutoCheck()
	}

	apiRouter := gin.New()
	apiRouter.Use(gin.Recovery())
	if *debugMode {
		apiRouter.Use(debugLogger)
	}
	apiRouter.Use(requestLogger(*debugMode))
	apiRouter.GET("/health", healthHandler)
	apiRouter.GET("/v1/models", proxyHandler.HandleModels)
	apiRouter.POST("/v1/chat/completions", proxyHandler.HandleChatCompletions)
	apiRouter.Any("/v1/", proxyHandler.HandleProxy)

	go func() {
		log.Printf("Starting API proxy server on port %d", *apiPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *apiPort), apiRouter); err != nil {
			log.Fatalf("API proxy server error: %v", err)
		}
	}()

	uiRouter := gin.New()
	tmpl := webui.LoadTemplate()
	uiRouter.SetHTMLTemplate(tmpl)
	if *debugMode {
		uiRouter.Use(uiDebugLogger)
	}

	registerUIRoutes(uiRouter)
	registerManagementRoutes(uiRouter, apiHandler)
	registerReloadRoute(uiRouter, apiHandler, proxyHandler, *configPath)

	log.Printf("Starting Web UI server on port %d", *uiPort)
	log.Printf("Open http://localhost:%d in your browser", *uiPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *uiPort), uiRouter); err != nil {
		log.Fatalf("Web UI server error: %v", err)
	}
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": version.Version,
	})
}

func registerUIRoutes(r *gin.Engine) {
	r.GET("/", pageHandler("首頁 / Home", webui.IndexFile))
	r.GET("/servers", pageHandler("服務器設置 / Servers", webui.ServersFile))
	r.GET("/server-models", pageHandler("模型設置 / Models", webui.ModelsFile))
	r.GET("/api-keys", pageHandler("API Key 設置 / API Keys", webui.APIKeysFile))
	r.GET("/local-models", pageHandler("本地模型映射 / Local Models", webui.LocalModelsFile))
	r.GET("/settings", pageHandler("系統設置 / Settings", webui.SettingsFile))
	r.GET("/test-results", pageHandler("測試結果 / Test Results", webui.TestResultsFile))
}

func pageHandler(title, pageFile string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   title,
			Content: webui.LoadPage(pageFile),
		})
	}
}

func registerManagementRoutes(r *gin.Engine, h *api.Handler) {
	r.GET("/api/servers", h.ListServers)
	r.POST("/api/servers", h.CreateServer)
	r.PUT("/api/servers", h.UpdateServer)
	r.DELETE("/api/servers/:id", h.DeleteServer)

	r.GET("/api/server-models", h.ListServerModels)
	r.POST("/api/server-models", h.CreateServerModel)
	r.PUT("/api/server-models", h.UpdateServerModel)
	r.DELETE("/api/server-models/:id", h.DeleteServerModel)

	r.GET("/api/server-api-keys", h.ListAPIKeys)
	r.POST("/api/server-api-keys", h.CreateAPIKey)
	r.PUT("/api/server-api-keys", h.UpdateAPIKey)
	r.DELETE("/api/server-api-keys/:id", h.DeleteAPIKey)

	r.GET("/api/pending-pool", h.ListPendingPool)

	r.GET("/api/local-model-maps", h.ListLocalModelMaps)
	r.POST("/api/local-model-maps", h.CreateLocalModelMap)
	r.PUT("/api/local-model-maps", h.UpdateLocalModelMap)
	r.DELETE("/api/local-model-maps/:id", h.DeleteLocalModelMap)

	r.GET("/api/version", h.GetVersion)
	r.GET("/api/settings", h.GetSettings)
	r.POST("/api/settings", h.UpdateSettings)

	r.POST("/api/reset-weights", h.ResetAllWeightsAllServers)
	r.POST("/api/reset-weights/:serverId", h.ResetWeights)

	r.POST("/api/test-key", h.TestSingleKey)
	r.GET("/api/test-key", h.TestBatchKeys)
	r.GET("/api/test-results", h.GetTestResults)
}

func registerReloadRoute(r *gin.Engine, h *api.Handler, ph *proxy.Handler, configPath string) {
	r.POST("/api/reload", func(c *gin.Context) {
		newStore, err := storage.NewStorage(configPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		store = newStore
		h.Store = store
		ph.Store = store
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func debugLogger(c *gin.Context) {
	start := time.Now()
	c.Next()
	log.Printf("[DEBUG] %s %s status=%d duration=%v",
		c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
}

func requestLogger(debug bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !debug {
			c.Next()
			return
		}
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		c.Next()
		duration := time.Since(start)
		status := c.Writer.Status()
		if path == "/v1/chat/completions" || path == "/v1/models" || c.Request.Header.Get("Authorization") != "" {
			log.Printf("[REQUEST] method=%s path=%s status=%d duration=%v client=%s",
				method, path, status, duration, c.ClientIP())
		}
	}
}

func uiDebugLogger(c *gin.Context) {
	start := time.Now()
	c.Next()
	log.Printf("[DEBUG-UI] %s %s status=%d duration=%v",
		c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
}

func runAutoCheck() {
	firstRun := true
	for {
		settings := store.GetSettings()
		interval := time.Duration(settings.AutoCheckIntervalHours) * time.Hour
		if interval < 1*time.Hour {
			interval = 6 * time.Hour
		} else if interval > 12*time.Hour {
			interval = 12 * time.Hour
		}

		if firstRun {
			delay := time.Duration(3+rand.Intn(13)) * time.Second
			log.Printf("[API_KEY_CHECK] Initial random delay: %v", delay)
			time.Sleep(delay)
			firstRun = false
		}

		log.Printf("[API_KEY_CHECK] Starting periodic API key check")
		checkAllAPIKeys()
		time.Sleep(interval)
	}
}

func checkAllAPIKeys() {
	servers := store.GetServers()
	for _, sv := range servers {
		keys := store.GetServerAPIKeysByServer(sv.ID)
		for _, key := range keys {
			if key.Status != "auto" {
				continue
			}
			time.Sleep(time.Duration(3+rand.Intn(13)) * time.Second)

			models := store.GetServerModelsByServer(sv.ID)
			testModelID := ""
			if len(models) > 0 {
				testModelID = models[0].ModelID
			}

			start := time.Now()
			result := api.TestSingleKeyExternal(sv.APIURL, key.APIKey, sv.APIType, testModelID)
			duration := time.Since(start)
			store.UpdateAPIKeyCheckResult(key.ID, result.Status, duration.String())
			log.Printf("[API_KEY_CHECK] key=%s server=%s result=%s duration=%v",
				maskKey(key.APIKey), sv.Name, result.Status, duration)
		}
	}
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	if len(key) <= 12 {
		return key[:3] + "****" + key[len(key)-2:]
	}
	return key[:3] + "****" + key[len(key)-4:]
}
