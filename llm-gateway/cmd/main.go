package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
	"llm_gateway/internal/version"
	"llm_gateway/internal/webui"

	"github.com/gin-gonic/gin"
)

var (
	configPath = flag.String("config", "config/config.yaml", "Path to config file")
	apiPort    = flag.Int("api-port", 18869, "API proxy port")
	uiPort     = flag.Int("ui-port", 18866, "Web UI port")
	debugMode  = flag.Bool("debug", false, "Enable debug logging")
)

var store *storage.Storage

var defaultTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 20,
	IdleConnTimeout:     90 * time.Second,
	DisableCompression:  false,
}

func createHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: defaultTransport,
	}
}

func updateHTTPClientTimeout() {
	settings := store.GetSettings()
	timeout := 5 * time.Minute
	if settings.Timeout > 0 {
		timeout = time.Duration(settings.Timeout) * time.Minute
	}
	defaultTransport.ResponseHeaderTimeout = timeout
	log.Printf("HTTP Client timeout set to %d minutes", settings.Timeout)
}

func createServerClient(serverTimeout int) *http.Client {
	settings := store.GetSettings()
	totalTimeout := settings.Timeout
	if serverTimeout > 0 {
		totalTimeout = serverTimeout
	}
	timeout := time.Duration(totalTimeout) * time.Minute
	return createHTTPClient(timeout)
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

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

	updateHTTPClientTimeout()

	// Background API key health checker
	settings := store.GetSettings()
	if settings.EnableAutoCheckAPIKey {
		go func() {
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
		}()
	}

	// --- API proxy server ---
	log.Printf("Starting API proxy server on port %d", *apiPort)

	apiRouter := gin.New()
	apiRouter.Use(gin.Recovery())

	if *debugMode {
		apiRouter.Use(func(c *gin.Context) {
			start := time.Now()
			c.Next()
			log.Printf("[DEBUG] %s %s status=%d duration=%v",
				c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
		})
	}

	apiRouter.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		if path == "/v1/chat/completions" || path == "/v1/models" || c.Request.Header.Get("Authorization") != "" {
			if *debugMode {
				log.Printf("[REQUEST] method=%s path=%s status=%d duration=%v client=%s",
					method, path, status, duration, c.ClientIP())
			}
		}
	})

	apiRouter.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": version.Version,
		})
	})

	apiRouter.GET("/v1/models", handleModels)
	apiRouter.POST("/v1/chat/completions", handleChatCompletions)
	apiRouter.Any("/v1/", handleProxy)

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *apiPort), apiRouter); err != nil {
			log.Fatalf("API proxy server error: %v", err)
		}
	}()

	// --- Web UI server ---
	log.Printf("Starting Web UI server on port %d", *uiPort)
	log.Printf("Open http://localhost:%d in your browser", *uiPort)

	uiRouter := gin.New()
	uiRouter.SetHTMLTemplate(webui.Templates)

	if *debugMode {
		uiRouter.Use(func(c *gin.Context) {
			start := time.Now()
			c.Next()
			log.Printf("[DEBUG-UI] %s %s status=%d duration=%v",
				c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
		})
	}

	// Static routes
	uiRouter.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "首頁 / Home",
			Content: template.HTML(webui.IndexPage),
		})
	})
	uiRouter.GET("/servers", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "服務器設置 / Servers",
			Content: template.HTML(webui.ServersPage),
		})
	})
	uiRouter.GET("/server-models", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "模型設置 / Models",
			Content: template.HTML(webui.ServerModelsPage),
		})
	})
	uiRouter.GET("/api-keys", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "API Key 設置 / API Keys",
			Content: template.HTML(webui.APIKeysPage),
		})
	})
	uiRouter.GET("/pending-pool", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "待選池 / Pending Pool",
			Content: template.HTML(webui.PendingPoolPage),
		})
	})
	uiRouter.GET("/local-models", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "本地模型映射 / Local Models",
			Content: template.HTML(webui.LocalModelsPage),
		})
	})
	uiRouter.GET("/settings", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "系統設置 / Settings",
			Content: template.HTML(webui.SettingsPage),
		})
	})
	uiRouter.GET("/test-results", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{
			Title:   "測試結果 / Test Results",
			Content: template.HTML(webui.TestResultPage),
		})
	})

	// --- Management API ---

	// Servers
	uiRouter.GET("/api/servers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"servers": store.GetServers()})
	})
	uiRouter.POST("/api/servers", func(c *gin.Context) {
		var sv storage.Server
		if err := c.ShouldBindJSON(&sv); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddServer(sv); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.PUT("/api/servers", func(c *gin.Context) {
		var sv storage.Server
		if err := c.ShouldBindJSON(&sv); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpdateServer(sv); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/servers/:id", func(c *gin.Context) {
		if err := store.DeleteServer(c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Server Models
	uiRouter.GET("/api/server-models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"server_models": store.GetServerModels()})
	})
	uiRouter.POST("/api/server-models", func(c *gin.Context) {
		var m storage.ServerModel
		if err := c.ShouldBindJSON(&m); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddServerModel(m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.PUT("/api/server-models", func(c *gin.Context) {
		var m storage.ServerModel
		if err := c.ShouldBindJSON(&m); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpdateServerModel(m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/server-models/:id", func(c *gin.Context) {
		if err := store.DeleteServerModel(c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Server API Keys
	uiRouter.GET("/api/server-api-keys", func(c *gin.Context) {
		keys := store.GetServerAPIKeys()
		masked := make([]storage.ServerAPIKey, len(keys))
		for i, k := range keys {
			masked[i] = k
			masked[i].APIKey = utils.MaskKey(k.APIKey)
		}
		c.JSON(http.StatusOK, gin.H{"server_api_keys": masked})
	})
	uiRouter.POST("/api/server-api-keys", func(c *gin.Context) {
		var k storage.ServerAPIKey
		if err := c.ShouldBindJSON(&k); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddServerAPIKey(k); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.PUT("/api/server-api-keys", func(c *gin.Context) {
		var k storage.ServerAPIKey
		if err := c.ShouldBindJSON(&k); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpdateServerAPIKey(k); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/server-api-keys/:id", func(c *gin.Context) {
		if err := store.DeleteServerAPIKey(c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Pending Pool API (read-only, pool is auto-computed) — grouped by server
	uiRouter.GET("/api/pending-pool", func(c *gin.Context) {
		servers := store.GetServers()
		result := make(map[string][]storage.ServerAPIKey)
		for _, sv := range servers {
			keys := store.GetPendingPoolKeys(sv.ID)
			if len(keys) > 0 {
				masked := make([]storage.ServerAPIKey, len(keys))
				for i, k := range keys {
					masked[i] = k
					masked[i].APIKey = utils.MaskKey(k.APIKey)
				}
				result[sv.ID] = masked
			}
		}
		c.JSON(http.StatusOK, gin.H{"servers": result})
	})

	// Local Model Maps
	uiRouter.GET("/api/local-model-maps", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"local_model_maps": store.GetLocalModelMaps()})
	})
	uiRouter.POST("/api/local-model-maps", func(c *gin.Context) {
		var lm storage.LocalModelMapping
		if err := c.ShouldBindJSON(&lm); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.AddLocalModelMap(lm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.PUT("/api/local-model-maps", func(c *gin.Context) {
		var lm storage.LocalModelMapping
		if err := c.ShouldBindJSON(&lm); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpdateLocalModelMap(lm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.DELETE("/api/local-model-maps/:id", func(c *gin.Context) {
		if err := store.DeleteLocalModelMap(c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Settings
	uiRouter.GET("/api/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": version.Version, "build_date": version.BuildDate})
	})
	uiRouter.GET("/api/settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, store.GetSettings())
	})
	uiRouter.POST("/api/settings", func(c *gin.Context) {
		var st storage.Settings
		if err := c.ShouldBindJSON(&st); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if st.AutoCheckIntervalHours < 1 {
			st.AutoCheckIntervalHours = 6
		} else if st.AutoCheckIntervalHours > 12 {
			st.AutoCheckIntervalHours = 12
		}
		if err := store.UpdateSettings(st); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updateHTTPClientTimeout()
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Reload
	uiRouter.POST("/api/reload", func(c *gin.Context) {
		newStore, err := storage.NewStorage(*configPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		store = newStore
		updateHTTPClientTimeout()
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Weight reset
	uiRouter.POST("/api/reset-weights", func(c *gin.Context) {
		if err := store.ResetAllWeightsAllServers(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	uiRouter.POST("/api/reset-weights/:serverId", func(c *gin.Context) {
		if err := store.ResetAllWeights(c.Param("serverId")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// --- API Key Test endpoints ---

	// In-memory test results (ephemeral, lost on restart - matches original behavior)
	var testResults []TestResult
	var testMu sync.Mutex

	// Single key test — accepts key_id to look up from storage
	uiRouter.POST("/api/test-key", func(c *gin.Context) {
		var req struct {
			KeyID string `json:"key_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.KeyID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key_id is required"})
			return
		}

		key, ok := store.GetServerAPIKeyByID(req.KeyID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
			return
		}
		sv, ok := store.GetServer(key.ServerID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}

		// Use the first model from this server's model list
		models := store.GetServerModelsByServer(key.ServerID)
		testModelID := ""
		if len(models) > 0 {
			testModelID = models[0].ModelID
		}

		result := testSingleKeyWithModel(sv.APIURL, key.APIKey, sv.APIType, testModelID)
		store.UpdateAPIKeyCheckResult(req.KeyID, result.Status, result.Duration)
		c.JSON(http.StatusOK, result)
	})

	// Batch test all keys for a server
	uiRouter.GET("/api/test-key", func(c *gin.Context) {
		serverID := c.Query("server_id")
		if serverID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "server_id is required"})
			return
		}

		sv, ok := store.GetServer(serverID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}

		if *debugMode {
			log.Printf("[DEBUG] test-key batch: server=%s api_url=%s api_type=%s", sv.Name, sv.APIURL, sv.APIType)
		}

		keys := store.GetServerAPIKeysByServer(serverID)
		pending := store.GetPendingPoolKeys(serverID)
		// Merge, dedup by ID
		seen := make(map[string]bool)
		var allKeys []storage.ServerAPIKey
		for _, k := range keys {
			if !seen[k.ID] {
				allKeys = append(allKeys, k)
				seen[k.ID] = true
			}
		}
		for _, k := range pending {
			if !seen[k.ID] {
				allKeys = append(allKeys, k)
				seen[k.ID] = true
			}
		}

		models := store.GetServerModelsByServer(serverID)
		var testModelID string
		if len(models) > 0 {
			testModelID = models[0].ModelID
		}

		if *debugMode {
			log.Printf("[DEBUG] test-key batch: found %d keys to test, testModel=%s", len(allKeys), testModelID)
		}

		testMu.Lock()
		testResults = nil
		testMu.Unlock()

		results := make([]TestResult, 0, len(allKeys))
		for _, key := range allKeys {
			time.Sleep(time.Duration(3+rand.Intn(13)) * time.Second)

			if *debugMode {
				log.Printf("[DEBUG] test-key: testing key %s with model=%s", key.ID, testModelID)
			}

			result := testSingleKeyWithModel(sv.APIURL, key.APIKey, sv.APIType, testModelID)
			result.KeyID = key.ID
			result.KeyMasked = utils.MaskKey(key.APIKey)
			results = append(results, result)

			if *debugMode {
				log.Printf("[DEBUG] test-key: key %s result=%s duration=%s", key.ID, result.Status, result.Duration)
			}

			store.UpdateAPIKeyCheckResult(key.ID, result.Status, result.Duration)
		}

		testMu.Lock()
		testResults = results
		testMu.Unlock()

		c.JSON(http.StatusOK, gin.H{"results": results})
	})

	// Get test results
	uiRouter.GET("/api/test-results", func(c *gin.Context) {
		testMu.Lock()
		results := make([]TestResult, len(testResults))
		copy(results, testResults)
		testMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"results": results})
	})

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *uiPort), uiRouter); err != nil {
		log.Fatalf("Web UI server error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Handler: /v1/models
// ---------------------------------------------------------------------------

func handleModels(c *gin.Context) {
	maps := store.GetLocalModelMaps()
	var data []gin.H
	for _, m := range maps {
		svModel, ok := store.GetServerModel(m.ServerModelID)
		if !ok {
			continue
		}
		sv, ok := store.GetServer(svModel.ServerID)
		if !ok {
			continue
		}
		data = append(data, gin.H{
			"id":       m.LocalModel,
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": sv.Name,
		})
	}
	if data == nil {
		data = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// ---------------------------------------------------------------------------
// Handler: /v1/chat/completions (core proxy)
// ---------------------------------------------------------------------------

type serverInfoResult struct {
	ModelID   string
	TargetURL string
	APIKey    string
	APIType   string
	ServerID  string
	Err       error
}

func getServerInfo(localModel string) serverInfoResult {
	modelID, targetURL, apiKey, apiType, serverID, err := store.GetServerInfo(localModel)
	return serverInfoResult{
		ModelID:   modelID,
		TargetURL: targetURL,
		APIKey:    apiKey,
		APIType:   apiType,
		ServerID:  serverID,
		Err:       err,
	}
}

func handleChatCompletions(c *gin.Context) {
	logAPIKey := true

	// Read body
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var reqBody struct {
		Model  string          `json:"model"`
		Stream bool            `json:"stream"`
		Raw    json.RawMessage `json:"-"` // passthrough for remaining fields
	}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Resolve model
	info := getServerInfo(reqBody.Model)
	if info.Err != nil {
		if logAPIKey {
			log.Printf("[CHAT] model=%s error=%v", reqBody.Model, info.Err)
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": info.Err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	settings := store.GetSettings()

	// Build upstream URL
	targetURL := strings.TrimRight(info.TargetURL, "/")

	// Determine chat completions path based on API type
	chatPath := "/chat/completions"
	switch info.APIType {
	case "anthropic":
		chatPath = "/v1/messages"
	case "ollama":
		chatPath = "/api/chat"
	default:
		if !strings.Contains(targetURL, "/v1") {
			chatPath = "/v1/chat/completions"
		} else {
			chatPath = "/chat/completions"
		}
	}

	var reqMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqMap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	reqMap["model"] = info.ModelID

	modifiedBody, err := json.Marshal(reqMap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if logAPIKey {
		log.Printf("[CHAT] model=%s → serverModel=%s server=%s key=%s",
			reqBody.Model, info.ModelID, info.TargetURL, utils.MaskKey(info.APIKey))
	}

	// Retry loop
	maxRetries := 1
	if settings.EnableRetry {
		maxRetries = 1 + settings.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			if logAPIKey {
				log.Printf("[CHAT] retry attempt %d/%d for model=%s", attempt, maxRetries-1, reqBody.Model)
			}
			// Get next key for retry
			currentKeyID := store.GetCurrentKey(info.ServerID)
			nextKey := store.GetNextKeyForServer(info.ServerID, currentKeyID)
			if nextKey == nil {
				break
			}
			info.APIKey = nextKey.APIKey
			store.SetCurrentKey(info.ServerID, nextKey.ID)
		}

		if *debugMode {
			log.Printf("[UPSTREAM_REQUEST] model=%s server=%s", reqBody.Model, info.TargetURL)
		}

		upstreamURL := targetURL + chatPath
		if strings.Contains(upstreamURL, "/v1/v1") {
			upstreamURL = strings.Replace(upstreamURL, "/v1/v1", "/v1", 1)
		}
		upReq, err := http.NewRequest(c.Request.Method, upstreamURL, bytes.NewReader(modifiedBody))
		if err != nil {
			lastErr = fmt.Errorf("create upstream request: %w", err)
			continue
		}

		// Copy headers
		upReq.Header.Set("Content-Type", "application/json")
		upReq.Header.Set("Authorization", "Bearer "+info.APIKey)
		if info.APIType == "anthropic" {
			upReq.Header.Set("x-api-key", info.APIKey)
			upReq.Header.Set("anthropic-version", "2023-06-01")
		}

		// Use context from client request
		upReq = upReq.WithContext(c.Request.Context())

		client := createServerClient(0)
		resp, err := client.Do(upReq)
		if err != nil {
			lastErr = err
			weight := classifyErrorAndGetWeight(err, settings)
			store.AddWeightToAPIKey(store.GetCurrentKey(info.ServerID), weight)
			store.ClearCurrentKey(info.ServerID)

			if settings.EnableRetry && isRetryableError(err, settings) {
				if logAPIKey {
					log.Printf("[CHAT] attempt=%d error=%v (retryable, weight=%d)", attempt, err, weight)
				}
				continue
			}
			if logAPIKey {
				log.Printf("[CHAT] attempt=%d error=%v (fatal, weight=%d)", attempt, err, weight)
			}
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":    "upstream_error",
				},
			})
			return
		}
		defer resp.Body.Close()

		// For non-streaming, read body and return
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 400 {
			// Weight the key
			weight := settings.Weight5xx
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				weight = settings.Weight4xx
			}
			store.AddWeightToAPIKey(store.GetCurrentKey(info.ServerID), weight)
			store.ClearCurrentKey(info.ServerID)

			if settings.EnableRetry && resp.StatusCode < 500 {
				// 4xx retry
				if logAPIKey {
					log.Printf("[CHAT] attempt=%d status=%d (retryable 4xx, weight=%d)", attempt, resp.StatusCode, weight)
				}
				continue
			}
			if settings.EnableRetry && resp.StatusCode >= 500 {
				if logAPIKey {
					log.Printf("[CHAT] attempt=%d status=%d (retryable 5xx, weight=%d)", attempt, resp.StatusCode, weight)
				}
				continue
			}

			// Non-retryable error
			for k, v := range resp.Header {
				for _, vv := range v {
					c.Header(k, vv)
				}
			}
			c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
			return
		}

		// Success
		if logAPIKey {
			log.Printf("[CHAT] attempt=%d status=%d model=%s duration=%dms",
				attempt, resp.StatusCode, reqBody.Model, 0)
		}

		for k, v := range resp.Header {
			for _, vv := range v {
				c.Header(k, vv)
			}
		}
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
		return
	}

	// All retries failed
	if lastErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": lastErr.Error(),
				"type":    "upstream_error",
			},
		})
	} else {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": "all retries exhausted",
				"type":    "upstream_error",
			},
		})
	}
}

// ---------------------------------------------------------------------------
// Handler: generic proxy for /v1/*
// ---------------------------------------------------------------------------

func handleProxy(c *gin.Context) {
	proxyPath := c.Param("path")
	if *debugMode {
		log.Printf("[UPSTREAM_REQUEST] proxy path=%s", proxyPath)
	}

	targetURL := c.Request.URL.String()
	upReq, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for k, v := range c.Request.Header {
		for _, vv := range v {
			upReq.Header.Add(k, vv)
		}
	}
	upReq = upReq.WithContext(c.Request.Context())

	if *debugMode {
		log.Printf("[UPSTREAM_CALL_START] proxy path=%s", proxyPath)
	}
	resp, err := http.DefaultTransport.RoundTrip(upReq)
	if *debugMode {
		if err != nil {
			log.Printf("[UPSTREAM_CALL_DONE] proxy path=%s err=%v", proxyPath, err)
		} else {
			log.Printf("[UPSTREAM_CALL_DONE] proxy path=%s status=%d", proxyPath, resp.StatusCode)
		}
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for k, v := range resp.Header {
		for _, vv := range v {
			c.Header(k, vv)
		}
	}
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// ---------------------------------------------------------------------------
// Error classification & retry helpers
// ---------------------------------------------------------------------------

func isRetryableError(err error, settings storage.Settings) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return settings.EnableRetryOnTimeout
		}
		if netErr.Temporary() {
			return true
		}
		return true // network error, retry
	}
	return false
}

func classifyErrorAndGetWeight(err error, settings storage.Settings) int {
	if err == nil {
		return 0
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" && netErr.Timeout() {
			return settings.ConnectTimeoutWeight
		}
		if netErr.Timeout() {
			return settings.TimeoutWeight
		}
		if netErr.Temporary() {
			if settings.TimeoutWeight > 1 {
				return settings.TimeoutWeight / 2
			}
			return 1
		}
		return settings.Weight5xx
	}
	return settings.Weight5xx
}

// ---------------------------------------------------------------------------
// API Key auto-health-check
// ---------------------------------------------------------------------------

func checkAllAPIKeys() {
	servers := store.GetServers()
	for _, sv := range servers {
		keys := store.GetServerAPIKeysByServer(sv.ID)
		for _, key := range keys {
			if key.Status != "auto" {
				continue
			}
			time.Sleep(time.Duration(3+rand.Intn(13)) * time.Second)

			// Use the server's first model instead of gpt-3.5-turbo fallback
			models := store.GetServerModelsByServer(sv.ID)
			testModelID := ""
			if len(models) > 0 {
				testModelID = models[0].ModelID
			}

			start := time.Now()
			result := testSingleKeyWithModel(sv.APIURL, key.APIKey, sv.APIType, testModelID)
			duration := time.Since(start)
			store.UpdateAPIKeyCheckResult(key.ID, result.Status, duration.String())
			log.Printf("[API_KEY_CHECK] key=%s server=%s result=%s duration=%v",
				utils.MaskKey(key.APIKey), sv.Name, result.Status, duration)
		}
	}
}

// ---------------------------------------------------------------------------
// TestResult
// ---------------------------------------------------------------------------

var testQuestions = []string{
	"What is 1+1?",
	"What is 2+2?",
	"What is 1+2?",
	"What is 3+1?",
	"What is 2+3?",
	"What is 1+3?",
	"What is 4+1?",
	"What is 5+1?",
	"What is 2+1?",
	"What is 1+4?",
	"What is 3+2?",
	"What is 1+5?",
	"What is 6+1?",
	"What is 7+1?",
	"What is 8+1?",
	"What is 1+6?",
	"What is 1+7?",
	"What is 1+8?",
	"What is 9+1?",
	"What is 10+1?",
}

type TestResult struct {
	KeyID     string `json:"key_id,omitempty"`
	KeyMasked string `json:"key_masked,omitempty"`
	Status    string `json:"status"`
	Duration  string `json:"duration"`
	Error     string `json:"error,omitempty"`
	ModelID   string `json:"model_id,omitempty"`
}

// testSingleKey uses chat completions (not GET models) to verify an API key.
// If no modelID is given, it's a single-key test and we use a best-effort model name.
func testSingleKey(apiURL, apiKey, apiType string) TestResult {
	return testSingleKeyWithModel(apiURL, apiKey, apiType, "")
}

func testSingleKeyWithModel(apiURL, apiKey, apiType, modelID string) TestResult {
	baseURL := strings.TrimRight(apiURL, "/")

	// Build chat URL like v1 does
	chatPath := "/chat/completions"
	if apiType == "anthropic" {
		chatPath = "/v1/messages"
	} else {
		// Ensure we have /v1/ prefix
		if !strings.Contains(baseURL, "/v1") {
			chatPath = "/v1/chat/completions"
		}
	}

	upstreamURL := baseURL + chatPath
	if strings.Contains(upstreamURL, "/v1/v1") {
		upstreamURL = strings.Replace(upstreamURL, "/v1/v1", "/v1", 1)
	}

	testModel := modelID
	if testModel == "" {
		testModel = "gpt-3.5-turbo" // fallback
	}

	randIdx := rand.Intn(len(testQuestions))
	testPrompt := testQuestions[randIdx]

	if *debugMode {
		log.Printf("[DEBUG] test-key: url=%s model=%s prompt=%s", upstreamURL, testModel, testPrompt)
	}

	payload := map[string]interface{}{
		"model": testModel,
		"messages": []map[string]string{
			{"role": "user", "content": testPrompt},
		},
		"max_tokens": 10,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		return TestResult{Status: "error", Duration: "0s", Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if apiType == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	if *debugMode {
		log.Printf("[UPSTREAM_CALL_START] test-key: url=%s model=%s", upstreamURL, testModel)
	}

	var testClient = createHTTPClient(30 * time.Second)
	start := time.Now()
	resp, err := testClient.Do(req)
	duration := time.Since(start)
	elapsed := time.Since(start)

	if *debugMode {
		if err != nil {
			log.Printf("[UPSTREAM_CALL_DONE] test-key: err=%v duration=%v", err, elapsed)
		} else {
			log.Printf("[UPSTREAM_CALL_DONE] test-key: status=%d duration=%v", resp.StatusCode, elapsed)
		}
	}

	if err != nil {
		return TestResult{Status: "error", Duration: duration.String(), Error: err.Error(), ModelID: testModel}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return TestResult{Status: "success", Duration: duration.String(), ModelID: testModel}
	}
	return TestResult{
		Status:   fmt.Sprintf("http_%d", resp.StatusCode),
		Duration: duration.String(),
		Error:    string(respBody),
		ModelID:  testModel,
	}
}