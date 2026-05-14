package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Store *storage.Storage
	Debug bool
}

var defaultTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 20,
	IdleConnTimeout:     90 * time.Second,
	DisableCompression:  false,
}

func createServerClient(settings storage.Settings, serverTimeout int) *http.Client {
	totalTimeout := settings.Timeout
	if serverTimeout > 0 {
		totalTimeout = serverTimeout
	}
	timeout := time.Duration(totalTimeout) * time.Minute
	return &http.Client{
		Timeout:   timeout,
		Transport: defaultTransport,
	}
}

// HandleModels returns the list of available local models.
func (h *Handler) HandleModels(c *gin.Context) {
	maps := h.Store.GetLocalModelMaps()
	var data []gin.H
	for _, m := range maps {
		svModel, ok := h.Store.GetServerModel(m.ServerModelID)
		if !ok {
			continue
		}
		sv, ok := h.Store.GetServer(svModel.ServerID)
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

// HandleChatCompletions proxies /v1/chat/completions with retry and streaming support.
func (h *Handler) HandleChatCompletions(c *gin.Context) {
	logAPIKey := true

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var reqBody struct {
		Model  string          `json:"model"`
		Stream bool            `json:"stream"`
	}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return
	}

	info := h.getServerInfo(reqBody.Model)
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

	// Build upstream URL
	targetURL := strings.TrimRight(info.TargetURL, "/")
	chatPath := getChatPath(info.APIType, targetURL)

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

	settings := h.Store.GetSettings()

	// Streaming path
	if reqBody.Stream {
		h.handleStreamRequest(c, info, targetURL, chatPath, modifiedBody, settings, logAPIKey)
		return
	}

	// Non-streaming with retry
	h.handleStandardRequest(c, info, targetURL, chatPath, modifiedBody, settings, logAPIKey)
}

func getChatPath(apiType, targetURL string) string {
	switch apiType {
	case "anthropic":
		return "/v1/messages"
	case "ollama":
		return "/api/chat"
	default:
		if !strings.Contains(targetURL, "/v1") {
			return "/v1/chat/completions"
		}
		return "/chat/completions"
	}
}

type serverInfoResult struct {
	ModelID   string
	TargetURL string
	APIKey    string
	APIType   string
	ServerID  string
	Err       error
}

func (h *Handler) getServerInfo(localModel string) serverInfoResult {
	modelID, targetURL, apiKey, apiType, serverID, err := h.Store.GetServerInfo(localModel)
	return serverInfoResult{
		ModelID:   modelID,
		TargetURL: targetURL,
		APIKey:    apiKey,
		APIType:   apiType,
		ServerID:  serverID,
		Err:       err,
	}
}

// handleStandardRequest processes non-streaming chat completions with retry.
func (h *Handler) handleStandardRequest(c *gin.Context, info serverInfoResult, targetURL, chatPath string, body []byte, settings storage.Settings, logAPIKey bool) {
	maxRetries := 1
	if settings.EnableRetry {
		maxRetries = 1 + settings.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			if logAPIKey {
				log.Printf("[CHAT] retry attempt %d/%d for model=%s", attempt, maxRetries-1, info.ModelID)
			}
			currentKeyID := h.Store.GetCurrentKey(info.ServerID)
			nextKey := h.Store.GetNextKeyForServer(info.ServerID, currentKeyID)
			if nextKey == nil {
				break
			}
			info.APIKey = nextKey.APIKey
			h.Store.SetCurrentKey(info.ServerID, nextKey.ID)
		}

		if h.Debug {
			log.Printf("[UPSTREAM_REQUEST] model=%s server=%s", info.ModelID, info.TargetURL)
		}

		upstreamURL := targetURL + chatPath
		upstreamURL = fixDuplicateV1(upstreamURL)

		upReq, err := http.NewRequest(c.Request.Method, upstreamURL, bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("create upstream request: %w", err)
			continue
		}

		setUpstreamHeaders(upReq, info)

		upReq = upReq.WithContext(c.Request.Context())

		client := createServerClient(settings, 0)
		resp, err := client.Do(upReq)
		if err != nil {
			lastErr = err
			weight := ClassifyErrorAndGetWeight(err, settings)
			h.Store.AddWeightToAPIKey(h.Store.GetCurrentKey(info.ServerID), weight)
			h.Store.ClearCurrentKey(info.ServerID)

			if settings.EnableRetry && IsRetryableError(err, settings) {
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

		// Read body synchronously (resp.Body closed via io.ReadAll)
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode >= 400 {
			weight := settings.Weight5xx
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				weight = settings.Weight4xx
			}
			h.Store.AddWeightToAPIKey(h.Store.GetCurrentKey(info.ServerID), weight)
			h.Store.ClearCurrentKey(info.ServerID)

			if settings.EnableRetry && resp.StatusCode < 500 {
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

			copyHeaders(c, resp)
			c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
			return
		}

		if logAPIKey {
			log.Printf("[CHAT] attempt=%d status=%d model=%s", attempt, resp.StatusCode, info.ModelID)
		}
		copyHeaders(c, resp)
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

// handleStreamRequest processes streaming chat completions (SSE) with retry.
func (h *Handler) handleStreamRequest(c *gin.Context, info serverInfoResult, targetURL, chatPath string, body []byte, settings storage.Settings, logAPIKey bool) {
	maxRetries := 1
	if settings.EnableRetry {
		maxRetries = 1 + settings.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			if logAPIKey {
				log.Printf("[STREAM] retry attempt %d/%d for model=%s", attempt, maxRetries-1, info.ModelID)
			}
			currentKeyID := h.Store.GetCurrentKey(info.ServerID)
			nextKey := h.Store.GetNextKeyForServer(info.ServerID, currentKeyID)
			if nextKey == nil {
				break
			}
			info.APIKey = nextKey.APIKey
			h.Store.SetCurrentKey(info.ServerID, nextKey.ID)
		}

		upstreamURL := targetURL + chatPath
		upstreamURL = fixDuplicateV1(upstreamURL)

		upReq, err := http.NewRequest(c.Request.Method, upstreamURL, bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("create upstream request: %w", err)
			continue
		}

		setUpstreamHeaders(upReq, info)
		upReq.Header.Set("Accept", "text/event-stream")
		upReq = upReq.WithContext(c.Request.Context())

		client := createServerClient(settings, 0)
		resp, err := client.Do(upReq)
		if err != nil {
			lastErr = err
			weight := ClassifyErrorAndGetWeight(err, settings)
			h.Store.AddWeightToAPIKey(h.Store.GetCurrentKey(info.ServerID), weight)
			h.Store.ClearCurrentKey(info.ServerID)

			if settings.EnableRetry && IsRetryableError(err, settings) {
				continue
			}
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":    "upstream_error",
				},
			})
			return
		}

		if resp.StatusCode >= 400 {
			// Read error body
			errBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			weight := settings.Weight5xx
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				weight = settings.Weight4xx
			}
			h.Store.AddWeightToAPIKey(h.Store.GetCurrentKey(info.ServerID), weight)
			h.Store.ClearCurrentKey(info.ServerID)

			if settings.EnableRetry && (resp.StatusCode < 500 || resp.StatusCode >= 500) {
				if logAPIKey {
					log.Printf("[STREAM] attempt=%d status=%d (retryable)", attempt, resp.StatusCode)
				}
				continue
			}

			c.Data(resp.StatusCode, "application/json", errBody)
			return
		}

		// Success: stream the response body as SSE
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(http.StatusOK)

		buf := make([]byte, 4096)
		flusher, canFlush := c.Writer.(http.Flusher)
		if !canFlush {
			// Fallback: read all and return
			allBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			c.Writer.Write(allBody)
			return
		}

		streamDone := make(chan struct{})
		go func() {
			defer resp.Body.Close()
			for {
				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
						return
					}
					flusher.Flush()
				}
				if readErr != nil {
					close(streamDone)
					return
				}
			}
		}()

		select {
		case <-streamDone:
			return
		case <-c.Request.Context().Done():
			resp.Body.Close()
			return
		}
	}

	// All retries exhausted
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

// HandleProxy is a generic passthrough proxy for /v1/* paths.
func (h *Handler) HandleProxy(c *gin.Context) {
	if h.Debug {
		log.Printf("[UPSTREAM_REQUEST] proxy path=%s", c.Param("path"))
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

	resp, err := http.DefaultTransport.RoundTrip(upReq)
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

	copyHeaders(c, resp)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setUpstreamHeaders(req *http.Request, info serverInfoResult) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+info.APIKey)
	if info.APIType == "anthropic" {
		req.Header.Set("x-api-key", info.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

func fixDuplicateV1(url string) string {
	if strings.Contains(url, "/v1/v1") {
		return strings.Replace(url, "/v1/v1", "/v1", 1)
	}
	return url
}

func copyHeaders(c *gin.Context, resp *http.Response) {
	for k, v := range resp.Header {
		for _, vv := range v {
			c.Header(k, vv)
		}
	}
}
