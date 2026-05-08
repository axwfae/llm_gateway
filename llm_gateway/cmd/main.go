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
	"net/url"
	"os"
	"strings"
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
	debugMode  = flag.Bool("debug", false, "Enable debug logging")
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

func main() {
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	
	if *debugMode || os.Getenv("DEBUG") == "true" {
		*debugMode = true
		gin.SetMode(gin.DebugMode)
	}
	
	log.Printf("LLM Gateway v%s (build: %s)", version.Version, version.BuildDate)
	log.Printf("Debug mode: %v", *debugMode)

	logAPIKey := true
	log.Printf("LOG_API_KEY enabled: %v", logAPIKey)

	var err error
	store, err = storage.NewStorage(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	updateHTTPClientTimeout()

	go func() {
		firstRun := true
		for {
			settings := store.GetSettings()
			interval := time.Duration(settings.AutoCheckIntervalHours) * time.Hour
			if interval < 3*time.Hour {
				interval = 6 * time.Hour
			}
			
			if firstRun {
				randomStartDelay := time.Duration(3+rand.Intn(13)) * time.Second
				log.Printf("[API_KEY_CHECK] Initial random delay: %v", randomStartDelay)
				time.Sleep(randomStartDelay)
				firstRun = false
			}
			
			log.Printf("[API_KEY_CHECK] Starting periodic API key check")
			checkAllAPIKeys()
			time.Sleep(interval)
		}
	}()

	log.Printf("Starting API server on port %d", *apiPort)
	
	apiRouter := gin.New()
	apiRouter.Use(gin.Recovery())
	
	if *debugMode {
		apiRouter.Use(func(c *gin.Context) {
			startTime := time.Now()
			
			c.Next()
			
			duration := time.Since(startTime)
			log.Printf("[DEBUG] %s %s status=%d duration=%v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
		})
	}

	apiRouter.Use(func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		
		c.Next()
		
		duration := time.Since(startTime)
		status := c.Writer.Status()
		
		if path == "/v1/chat/completions" || path == "/v1/models" || c.Request.Header.Get("Authorization") != "" {
			if *debugMode {
				log.Printf("[REQUEST] method=%s path=%s status=%d duration=%v client=%s model=%s", 
					method, path, status, duration, c.ClientIP(), c.GetHeader("X-Forwarded-For"))
			}
		}
	})

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

	uiRouter := gin.New()
	uiRouter.SetHTMLTemplate(webui.Templates)

	uiRouter.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "首頁", Content: template.HTML(webui.IndexPage)})
	})
	uiRouter.GET("/test-results", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main", webui.PageData{Title: "測試結果", Content: template.HTML(webui.TestResultPage)})
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
	uiRouter.PUT("/api/servers", func(c *gin.Context) {
		var server storage.Server
		if err := c.ShouldBindJSON(&server); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpdateServer(server); err != nil {
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
	uiRouter.PUT("/api/server-models", func(c *gin.Context) {
		var model storage.ServerModel
		if err := c.ShouldBindJSON(&model); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpdateServerModel(model); err != nil {
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
			masked[i] = gin.H{
				"id":               k.ID,
				"server_id":        k.ServerID,
				"api_key":          utils.MaskKey(k.APIKey),
				"is_active":        k.IsActive,
				"status":           k.Status,
				"notes":            k.Notes,
				"negative_weight":  k.NegativeWeight,
				"last_reset_time":  k.LastResetTime.Unix(),
				"last_check_time": k.LastCheckTime.Unix(),
				"last_check_result": k.LastCheckResult,
			}
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
	uiRouter.PUT("/api/server-api-keys", func(c *gin.Context) {
		var key storage.ServerAPIKey
		if err := c.ShouldBindJSON(&key); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existing := store.GetServerAPIKey(key.ID)
		if existing == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
			return
		}
		existing.Status = key.Status
		existing.Notes = key.Notes
		if err := store.UpdateServerAPIKey(*existing); err != nil {
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
	uiRouter.PUT("/api/local-model-maps", func(c *gin.Context) {
		var mapping storage.LocalModelMapping
		if err := c.ShouldBindJSON(&mapping); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpdateLocalModelMap(mapping); err != nil {
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

	uiRouter.POST("/api/reset-weights", func(c *gin.Context) {
		if err := store.ResetAllWeightsAllServers(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "所有負權重已重置"})
	})

	uiRouter.POST("/api/reset-weights/:serverId", func(c *gin.Context) {
		serverId := c.Param("serverId")
		if err := store.ResetAllWeights(serverId); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "負權重已重置"})
	})

	uiRouter.POST("/api/test-key", func(c *gin.Context) {
		var testReq struct {
			ServerID string `json:"server_id"`
			APIKey   string `json:"api_key"`
		}
		if err := c.ShouldBindJSON(&testReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		server := store.GetServer(testReq.ServerID)
		if server == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
			return
		}
		
		testAPIKey := testReq.APIKey
		if testAPIKey == "" {
			key := store.GetNextAPIKey(server.ID)
			if key == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "no api key available"})
				return
			}
			testAPIKey = key.APIKey
		}
		
		client := &http.Client{Timeout: 30 * time.Second}
		
		req, err := http.NewRequest("GET", server.APIURL+"/models", nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
		
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status": "error",
				"message": err.Error(),
				"valid": false,
			})
			return
		}
		defer resp.Body.Close()
		
		body, _ := io.ReadAll(resp.Body)
		
		if resp.StatusCode == 200 {
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
				"message": "API key is valid",
				"valid": true,
				"status_code": resp.StatusCode,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"status": "error",
				"message": string(body),
				"valid": false,
				"status_code": resp.StatusCode,
			})
		}
	})

	type testResult struct {
		KeyID     string `json:"key_id"`
		ServerID  string `json:"server_id"`
		APIKey    string `json:"api_key"`
		Status    string `json:"status"`
		Success   bool   `json:"success"`
		HTTPStatus int   `json:"http_status"`
		Timestamp int64  `json:"timestamp"`
	}
	var testResults []testResult

	uiRouter.GET("/api/test-key", func(c *gin.Context) {
		keyID := c.Query("key_id")
		if *debugMode {
			log.Printf("[DEBUG] test-key request: key_id=%s", keyID)
		}
		keys := store.GetServerAPIKeys()
		if *debugMode {
			log.Printf("[DEBUG] test-key: total keys in system: %d", len(keys))
			for i, k := range keys {
				log.Printf("[DEBUG] test-key: key[%d] id=%s status=%s", i, k.ID, k.Status)
			}
		}
		keysToTest := make([]storage.ServerAPIKey, 0)
		if keyID == "" {
			for _, k := range keys {
				if k.Status == "enabled" || k.Status == "auto" || k.Status == "" {
					keysToTest = append(keysToTest, k)
				}
			}
		} else {
			for _, k := range keys {
				if k.ID == keyID {
					keysToTest = append(keysToTest, k)
					break
				}
			}
		}
		if *debugMode {
			log.Printf("[DEBUG] test-key: found %d keys to test", len(keysToTest))
		}
		if len(keysToTest) == 0 {
			c.JSON(http.StatusOK, gin.H{"results": []testResult{}})
			return
		}
		client := &http.Client{Timeout: 30 * time.Second}
		now := time.Now().Unix()
		for _, key := range keysToTest {
			server := store.GetServer(key.ServerID)
			if server == nil {
				if *debugMode {
					log.Printf("[DEBUG] test-key: server not found for key %s", key.ID)
				}
continue
			}

			testQuestions := []string{
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
			randIdx := rand.Intn(len(testQuestions))
			testPrompt := testQuestions[randIdx]
			
			if *debugMode {
				log.Printf("[DEBUG] test-key: testing key %s with prompt: %s", key.ID, testPrompt)
			}
			
			serverModels := store.GetServerModels()
			testModelID := "test"
			for _, sm := range serverModels {
				if sm.ServerID == key.ServerID {
					testModelID = sm.ModelID
					break
				}
			}
			
			jsonBody := fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"%s"}],"max_tokens":10}`, testModelID, testPrompt)
			reqBody := bytes.NewBufferString(jsonBody)
			chatPath := server.APIURL
			if !strings.HasSuffix(chatPath, "/chat/completions") {
				if !strings.HasSuffix(chatPath, "/v1") {
					chatPath = chatPath + "/v1"
				}
				chatPath = chatPath + "/chat/completions"
			} else {
				if !strings.Contains(chatPath, "/v1") {
					chatPath = strings.Replace(chatPath, "/chat/completions", "/v1/chat/completions", 1)
				}
			}
			req, err := http.NewRequest("POST", chatPath, reqBody)
			if err != nil {
				if *debugMode {
					log.Printf("[DEBUG] test-key: failed to create request for key %s: %v", key.ID, err)
				}
				testResults = append(testResults, testResult{KeyID: key.ID, ServerID: key.ServerID, APIKey: key.APIKey, Status: key.Status, Success: false, HTTPStatus: 0, Timestamp: now})
				continue
			}
			req.Header.Set("Authorization", "Bearer "+key.APIKey)
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			success := false
			httpStatus := 0
			if err == nil {
				httpStatus = resp.StatusCode
				success = resp.StatusCode == 200
				resp.Body.Close()
			}
			if *debugMode {
				log.Printf("[DEBUG] test-key: key %s result: success=%v http_status=%d model=%s", key.ID, success, httpStatus, testModelID)
			}
			testResults = append(testResults, testResult{KeyID: key.ID, ServerID: key.ServerID, APIKey: key.APIKey, Status: key.Status, Success: success, HTTPStatus: httpStatus, Timestamp: now})
			store.UpdateAPIKeyCheckResult(key.ID, success)
			time.Sleep(time.Duration(3+rand.Intn(13)) * time.Second)
		}
		c.JSON(http.StatusOK, gin.H{"results": testResults})
	})
	uiRouter.GET("/api/test-key-results", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"results": testResults})
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
		resetDuration := time.Duration(settings.WeightResetHours) * time.Hour

		lowestWeight := int64(1<<63 - 1)
		for i := range keys {
			if keys[i].ID == currentKeyID {
				continue
			}
			if keys[i].NegativeWeight < lowestWeight {
				lowestWeight = keys[i].NegativeWeight
			}
		}

		var bestKeys []storage.ServerAPIKey
		for i := range keys {
			if keys[i].ID != currentKeyID && keys[i].NegativeWeight == lowestWeight {
				bestKeys = append(bestKeys, keys[i])
			}
		}

		if len(bestKeys) == 0 {
			return nil
		}

		if len(bestKeys) == 1 {
			return &bestKeys[0]
		}

		var expiredKeys []storage.ServerAPIKey
		var activeKeys []storage.ServerAPIKey

		for i := range bestKeys {
			if bestKeys[i].LastResetTime.IsZero() || time.Since(bestKeys[i].LastResetTime) >= resetDuration {
				expiredKeys = append(expiredKeys, bestKeys[i])
			} else {
				activeKeys = append(activeKeys, bestKeys[i])
			}
		}

		var candidates []storage.ServerAPIKey
		if len(expiredKeys) > 0 {
			candidates = expiredKeys
		} else {
			candidates = bestKeys
		}

		longestReset := candidates[0].LastResetTime
		for i := 1; i < len(candidates); i++ {
			if candidates[i].LastResetTime.After(longestReset) {
				longestReset = candidates[i].LastResetTime
			}
		}

		var longestKeys []storage.ServerAPIKey
		for i := range candidates {
			if candidates[i].LastResetTime.Equal(longestReset) {
				longestKeys = append(longestKeys, candidates[i])
			}
		}

		idx := rand.Intn(len(longestKeys))
		return &longestKeys[idx]
	}

	for i, k := range keys {
		if k.ID != currentKeyID {
			return &keys[i]
		}
	}
	return nil
}

func checkAllAPIKeys() {
	serversWithKeys := store.GetServersWithAPIKeys()
	client := &http.Client{Timeout: 30 * time.Second}
	
	for _, data := range serversWithKeys {
		key := data.ServerAPIKey
		if key.Status != "auto" {
			continue
		}
		
		server := data.Server
		req, err := http.NewRequest("GET", server.APIURL+"/v1/models", nil)
		if err != nil {
			log.Printf("[API_KEY_CHECK] Failed to create request for key %s: %v", key.ID, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+key.APIKey)
		
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[API_KEY_CHECK] Key %s check failed: %v", key.ID, err)
			store.UpdateAPIKeyCheckResult(key.ID, false)
			continue
		}
		defer resp.Body.Close()
		
		result := resp.StatusCode == 200
		log.Printf("[API_KEY_CHECK] Key %s check result: %v (status: %d)", key.ID, result, resp.StatusCode)
		store.UpdateAPIKeyCheckResult(key.ID, result)
		
		randomDelay := time.Duration(3+rand.Intn(13)) * time.Second
		time.Sleep(randomDelay)
	}
}

func handleChatCompletions(c *gin.Context, logAPIKey bool) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[REQUEST_ERROR] chat/completions invalid body error=%v client=%s", err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var originalBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &originalBody); err != nil {
		log.Printf("[REQUEST_ERROR] chat/completions invalid json error=%v client=%s", err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	localModel, ok := originalBody["model"].(string)
	if !ok {
		log.Printf("[REQUEST_ERROR] chat/completions model not specified client=%s", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "model not specified"})
		return
	}

	log.Printf("[REQUEST_START] chat/completions model=%s client=%s", localModel, c.ClientIP())
	
	info, err := getServerInfo(localModel)
	if err != nil {
		log.Printf("[REQUEST_ERROR] chat/completions model=%s error=%v client=%s", localModel, err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings := store.GetSettings()
	
	if *debugMode || os.Getenv("DEBUG") == "true" {
		log.Printf("[DEBUG] after getServerInfo: model=%s server=%s", localModel, info.Server.Name)
	}

	maxRetries := 0
	if settings.EnableRetry {
		maxRetries = settings.MaxRetries
	}

	timeout := settings.Timeout
	if info.Server.Timeout > 0 {
		timeout = info.Server.Timeout
	}
	totalTimeout := time.Duration(timeout) * time.Minute
	
	var lastError error
	var respBody []byte
	var respStatusCode int

	if *debugMode {
		log.Printf("[UPSTREAM_REQUEST] server=%s model=%s url=%s", info.Server.Name, localModel, info.Server.APIURL)
		log.Printf("[NET_CHECK] target=%s timeout=%v", info.Server.APIURL, totalTimeout)
	}
	
	startTime := time.Now()
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if *debugMode {
			log.Printf("[UPSTREAM_CALL_START] model=%s attempt=%d url=%s", localModel, attempt, info.Server.APIURL+"/chat/completions")
		}
		
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

		client := createServerHTTPClient(info.Server, settings)
		
		if *debugMode {
			log.Printf("[UPSTREAM_DO_START] model=%s server=%s url=%s", localModel, info.Server.Name, info.Server.APIURL+"/chat/completions")
		}
		resp, err := client.Do(proxyReq)
		if *debugMode {
			log.Printf("[UPSTREAM_DO_DONE] model=%s server=%s err=%v", localModel, info.Server.Name, err)
		}
		
		if err != nil {
			lastError = err
             
			weight := classifyErrorAndGetWeight(err, settings)
			log.Printf("[UPSTREAM_ERROR] server=%s model=%s error_type=%s weight=%d attempt=%d",
				info.Server.Name, localModel, getErrorType(err), weight, attempt)
            
			if settings.EnableNegativeWeight {
				store.AddWeightToAPIKey(info.APIKey.ID, weight)
				store.ClearCurrentKey(info.Server.ID)
			}
			
			if isRetryableError(err, settings) && attempt < maxRetries {
				continue
			}
			break
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
	
	duration := time.Since(startTime)
	if *debugMode {
		log.Printf("[REQUEST_COMPLETE] model=%s server=%s status=%d duration=%v", 
			localModel, info.Server.Name, respStatusCode, duration)
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

func createServerHTTPClient(server *storage.Server, settings storage.Settings) *http.Client {
	timeout := settings.Timeout
	if server.Timeout > 0 {
		timeout = server.Timeout
	}
	totalTimeout := time.Duration(timeout) * time.Minute
	connectTimeout := time.Duration(settings.ConnectTimeout) * time.Second
	
	if *debugMode {
		log.Printf("[CLIENT_CONFIG] server=%s totalTimeout=%v connectTimeout=%s responseHeaderTimeout=%v",
			server.Name, totalTimeout, connectTimeout, totalTimeout-connectTimeout)
	}
	
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: connectTimeout,
			}).DialContext,
			TLSHandshakeTimeout: connectTimeout,
			ResponseHeaderTimeout: totalTimeout - connectTimeout,
		},
		Timeout: totalTimeout,
	}
}

func getErrorType(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			if _, ok := netErr.(*net.OpError); ok {
				op := netErr.(*net.OpError)
				if op.Op == "dial" {
					return "connect_timeout"
				}
				return "timeout"
			}
			return "timeout"
		}
		if netErr.Temporary() {
			return "temporary"
		}
		return "network_error"
	}
	return "unknown_error"
}

func classifyErrorAndGetWeight(err error, settings storage.Settings) int {
	errorType := getErrorType(err)
	switch errorType {
	case "connect_timeout":
		return settings.ConnectTimeoutWeight
	case "timeout":
		return settings.TimeoutWeight
	case "temporary":
		return settings.TimeoutWeight / 2
	case "network_error":
		return settings.Weight5xx
	default:
		return settings.Weight5xx
	}
}

func isRetryableError(err error, settings storage.Settings) bool {
	if !settings.EnableRetry {
		return false
	}
	
	errorType := getErrorType(err)
	
	switch errorType {
	case "connect_timeout":
		return settings.EnableRetryOnTimeout
	case "timeout":
		return settings.EnableRetryOnTimeout
	case "temporary":
		return true
	case "network_error":
		return true
	default:
		return settings.EnableRetry
	}
}