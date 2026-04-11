package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
	"llm_gateway/internal/version"
	"llm_gateway/internal/webui"

	"github.com/gin-gonic/gin"
)

var (
	configPath = flag.String("config", "config/config.yaml", "Path to config file")
	apiPort    = flag.Int("api-port", 18869, "API port")
	uiPort     = flag.Int("ui-port", 18866, "Web UI port")
)

var store *storage.Storage

var httpClient = &http.Client{}

func updateHTTPClientTimeout() {
	settings := store.GetSettings()
	if settings.Timeout > 0 {
		httpClient.Timeout = time.Duration(settings.Timeout) * time.Minute
	} else {
		httpClient.Timeout = 5 * time.Minute
	}
	log.Printf("HTTP Client timeout set to %d minutes", settings.Timeout)
}

func startWeightResetTimer() {
	settings := store.GetSettings()
	if !settings.EnableNegativeWeight || settings.WeightResetHours <= 0 {
		return
	}

	duration := time.Duration(settings.WeightResetHours) * time.Hour
	ticker := time.NewTicker(duration)
	go func() {
		for range ticker.C {
			if err := store.ResetAllWeightsAllServers(); err != nil {
				log.Printf("[WEIGHT_RESET] Failed: %v", err)
			} else {
				log.Printf("[WEIGHT_RESET] All API key weights reset to 0")
			}
		}
	}()
	log.Printf("[WEIGHT_RESET] Timer started, reset every %d hours", settings.WeightResetHours)
}

func main() {
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	log.Printf("LLM Gateway v%s (build: %s)", version.Version, version.BuildDate)

	logAPIKey := true
	log.Printf("LOG_API_KEY enabled: %v", logAPIKey)

	var err error
	store, err = storage.NewStorage(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	updateHTTPClientTimeout()

	startWeightResetTimer()

	log.Printf("Starting API server on port %d", *apiPort)
	apiRouter := gin.New()
	apiRouter.Use(gin.Recovery())

	apiRouter.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version.Version})
	})

	apiRouter.GET("/v1/models", func(c *gin.Context) {
		handleModels(c)
	})

	apiRouter.POST("/v1/chat/completions", func(c *gin.Context) {
		log.Printf("Received chat/completions request")
		handleChatCompletions(c, logAPIKey)
	})
	apiRouter.Any("/v1/", func(c *gin.Context) {
		log.Printf("Received proxy request: %s %s", c.Request.Method, c.Request.URL.Path)
		handleProxy(c, logAPIKey)
	})

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *apiPort), apiRouter); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	log.Printf("Starting Web UI on port %d", *uiPort)
	log.Printf("Open http://localhost:%d in your browser", *uiPort)

	uiRouter := gin.Default()
	uiRouter.SetHTMLTemplate(webui.Templates)

	uiRouter.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "首頁", Content: template.HTML(webui.IndexPage)})
	})
	uiRouter.GET("/servers", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "服務器設置", Content: template.HTML(webui.ServersPage)})
	})
	uiRouter.GET("/server-models", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "服務器模型設置", Content: template.HTML(webui.ServerModelsPage)})
	})
	uiRouter.GET("/api-keys", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "API Key 設置", Content: template.HTML(webui.APIKeysPage)})
	})
	uiRouter.GET("/local-models", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "本地模型映射", Content: template.HTML(webui.LocalModelsPage)})
	})
	uiRouter.GET("/settings", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "系統設置", Content: template.HTML(webui.SettingsPage)})
	})

	uiRouter.GET("/api/servers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"servers": store.GetServers()})
	})
	uiRouter.POST("/api/servers", func(c *gin.Context) {
		var server storage.Server
		if err := c.ShouldBindJSON(&server); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddServer(server); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/servers/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := store.DeleteServer(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	uiRouter.GET("/api/server-models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"server_models": store.GetServerModels()})
	})
	uiRouter.POST("/api/server-models", func(c *gin.Context) {
		var model storage.ServerModel
		if err := c.ShouldBindJSON(&model); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddServerModel(model); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/server-models/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := store.DeleteServerModel(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	uiRouter.GET("/api/server-api-keys", func(c *gin.Context) {
		keys := store.GetServerAPIKeys()
		masked := make([]gin.H, len(keys))
		for i, k := range keys {
			masked[i] = gin.H{"id": k.ID, "server_id": k.ServerID, "api_key": utils.MaskKey(k.APIKey), "is_active": k.IsActive, "negative_weight": k.NegativeWeight}
		}
		c.JSON(http.StatusOK, gin.H{"server_api_keys": masked})
	})
	uiRouter.POST("/api/server-api-keys", func(c *gin.Context) {
		var key storage.ServerAPIKey
		if err := c.ShouldBindJSON(&key); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddServerAPIKey(key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/server-api-keys/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := store.DeleteServerAPIKey(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	uiRouter.GET("/api/local-model-maps", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"local_model_maps": store.GetLocalModelMaps()})
	})
	uiRouter.POST("/api/local-model-maps", func(c *gin.Context) {
		var mapping storage.LocalModelMapping
		if err := c.ShouldBindJSON(&mapping); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddLocalModelMap(mapping); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/local-model-maps/:id", func(c *gin.Context) {
		id := c.Param("id")
		if err := store.DeleteLocalModelMap(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	uiRouter.POST("/api/reload", func(c *gin.Context) {
		newStore, err := storage.NewStorage(*configPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		store = newStore
		updateHTTPClientTimeout()
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "配置已重新載入"})
	})

	uiRouter.GET("/api/settings", func(c *gin.Context) {
		settings := store.GetSettings()
		c.JSON(http.StatusOK, gin.H{
			"settings": settings,
			"version": version.Version,
			"ports":   gin.H{"api": *apiPort, "ui": *uiPort},
		})
	})
	uiRouter.POST("/api/settings", func(c *gin.Context) {
		var settings storage.Settings
		if err := c.ShouldBindJSON(&settings); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if settings.Timeout < 3 {
			settings.Timeout = 3
		}
		if settings.Timeout > 15 {
			settings.Timeout = 15
		}
		if settings.WeightResetHours > 0 {
			if settings.WeightResetHours < 2 {
				settings.WeightResetHours = 2
			}
			if settings.WeightResetHours > 8 {
				settings.WeightResetHours = 8
			}
		}
		if err := store.UpdateSettings(settings); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updateHTTPClientTimeout()
		startWeightResetTimer()
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *uiPort), uiRouter); err != nil {
		log.Fatalf("UI server error: %v", err)
	}
}

type ServerInfo struct {
	Server     *storage.Server
	ServerModel *storage.ServerModel
	APIKey     *storage.ServerAPIKey
}

func getServerInfo(localModel string) (*ServerInfo, error) {
	mapping := store.GetLocalModelMap(localModel)
	if mapping == nil {
		return nil, fmt.Errorf("model not mapped")
	}

	serverModel := store.GetServerModel(mapping.ServerModelID)
	if serverModel == nil {
		return nil, fmt.Errorf("server model not found")
	}

	server := store.GetServer(serverModel.ServerID)
	if server == nil {
		return nil, fmt.Errorf("server not found")
	}

	apiKey := store.GetNextAPIKey(server.ID)
	if apiKey == nil {
		return nil, fmt.Errorf("no api key available")
	}

	return &ServerInfo{
		Server:     server,
		ServerModel: serverModel,
		APIKey:     apiKey,
	}, nil
}

func getNextKeyForServer(serverID string, currentKeyID string) *storage.ServerAPIKey {
	keys := store.GetServerAPIKeysByServer(serverID)
	if len(keys) == 0 {
		return nil
	}

	settings := store.GetSettings()
	if settings.EnableNegativeWeight {
		lowestWeight := int64(1<<63 - 1)
		for i := range keys {
			if keys[i].ID == currentKeyID {
				continue
			}
			if keys[i].NegativeWeight < lowestWeight {
				lowestWeight = keys[i].NegativeWeight
			}
		}

		var lowestKeys []storage.ServerAPIKey
		for i := range keys {
			if keys[i].ID != currentKeyID && keys[i].NegativeWeight == lowestWeight {
				lowestKeys = append(lowestKeys, keys[i])
			}
		}

		if len(lowestKeys) == 0 {
			return nil
		}

		idx := rand.Intn(len(lowestKeys))
		return &lowestKeys[idx]
	}

	for i, k := range keys {
		if k.ID != currentKeyID {
			return &keys[i]
		}
	}
	return nil
}

func handleChatCompletions(c *gin.Context, logAPIKey bool) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var originalBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &originalBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	localModel, ok := originalBody["model"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model not specified"})
		return
	}

	info, err := getServerInfo(localModel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings := store.GetSettings()

	maxRetries := 0
	if settings.EnableRetry {
		maxRetries = settings.MaxRetries
	}

	var lastError error
	var respBody []byte
	var respStatusCode int

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if logAPIKey {
				log.Printf("[RETRY] attempt=%d model=%s server=%s", attempt, localModel, info.Server.Name)
			}
			nextKey := getNextKeyForServer(info.Server.ID, info.APIKey.ID)
			if nextKey == nil {
				break
			}
			info.APIKey = nextKey
		}
		if logAPIKey {
			log.Printf("[API_KEY_ROTATION] Using API key: %s for model: %s, server: %s (attempt=%d)", utils.MaskKey(info.APIKey.APIKey), localModel, info.Server.Name, attempt)
		}

		originalBody["model"] = info.ServerModel.ModelID
		newBody, err := json.Marshal(originalBody)
		if err != nil {
			lastError = err
			continue
		}

		proxyReq, err := http.NewRequest("POST", info.Server.APIURL+"/chat/completions", bytes.NewReader(newBody))
		if err != nil {
			lastError = err
			continue
		}

		proxyReq.Header.Set("Content-Type", "application/json")
		proxyReq.Header.Set("Authorization", "Bearer "+info.APIKey.APIKey)
		proxyReq.ContentLength = int64(len(newBody))

		client := httpClient
		resp, err := client.Do(proxyReq)
		if err != nil {
			lastError = err
			log.Printf("[UPSTREAM_ERROR] server=%s model=%s error=%v attempt=%d", info.Server.Name, localModel, err, attempt)
			if settings.EnableNegativeWeight {
				store.AddWeightToAPIKey(info.APIKey.ID, settings.Weight5xx)
				store.ClearCurrentKey(info.Server.ID)
			}
			continue
		}

		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			respBody = []byte{}
		}

		respStatusCode = resp.StatusCode

		if resp.StatusCode >= 400 {
			log.Printf("[UPSTREAM_ERROR] server=%s model=%s status=%d attempt=%d body=%s",
				info.Server.Name, localModel, resp.StatusCode, attempt, truncateLog(string(respBody), 500))

			weight := settings.Weight4xx
			if resp.StatusCode >= 500 {
				weight = settings.Weight5xx
			}
			if settings.EnableNegativeWeight {
				store.AddWeightToAPIKey(info.APIKey.ID, weight)
				store.ClearCurrentKey(info.Server.ID)
			}

			if attempt < maxRetries && resp.StatusCode < 500 {
				continue
			}
		}

		break
	}

	if respStatusCode == 0 {
		if lastError != nil {
			log.Printf("[ALL_RETRY_FAILED] model=%s error=%v", localModel, lastError)
			c.JSON(http.StatusInternalServerError, gin.H{"error": lastError.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unknown error"})
		}
		return
	}

	c.Header("Content-Type", "application/json")
	c.Status(respStatusCode)
	c.Writer.Write(respBody)
}

func handleProxy(c *gin.Context, logAPIKey bool) {
	localModel := c.Query("model")
	if localModel == "" {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil {
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			var req struct{ Model string `json:"model"` }
			if json.Unmarshal(bodyBytes, &req) == nil {
				localModel = req.Model
			}
		}
	}

	info, err := getServerInfo(localModel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if logAPIKey {
		log.Printf("[API_KEY_ROTATION] Using API key: %s for model: %s, server: %s", utils.MaskKey(info.APIKey.APIKey), localModel, info.Server.Name)
	}

	target, err := url.Parse(info.Server.APIURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid server URL"})
		return
	}

	c.Request.Header.Set("Authorization", "Bearer "+info.APIKey.APIKey)
	c.Request.URL.Scheme = target.Scheme
	c.Request.URL.Host = target.Host
	c.Request.URL.Path = "/" + c.Param("path")

	resp, err := http.DefaultTransport.RoundTrip(c.Request)
	if err == nil && resp != nil && resp.StatusCode >= 400 {
		log.Printf("[UPSTREAM_ERROR] server=%s model=%s status=%d",
			info.Server.Name, localModel, resp.StatusCode)
	}
}

func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func handleModels(c *gin.Context) {
	maps := store.GetLocalModelMaps()
	models := store.GetServerModels()

	var result struct {
		Data []struct {
			ID         string `json:"id"`
			Object     string `json:"object"`
			Created   int    `json:"created"`
			OwnedBy   string `json:"owned_by"`
		} `json:"data"`
	}

	for _, m := range maps {
		var serverName string
		for _, sm := range models {
			if sm.ID == m.ServerModelID {
				serverName = sm.ModelName
				break
			}
		}
		result.Data = append(result.Data, struct {
			ID         string `json:"id"`
			Object     string `json:"object"`
			Created   int    `json:"created"`
			OwnedBy   string `json:"owned_by"`
		}{
			ID:       m.LocalModel,
			Object:   "model",
			Created:  0,
			OwnedBy:  serverName,
		})
	}

	c.JSON(http.StatusOK, result)
}