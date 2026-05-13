package api

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"llm_gateway/internal/utils"

	"github.com/gin-gonic/gin"
	llm_storage "llm_gateway/internal/storage"
)

func (h *Handler) TestSingleKey(c *gin.Context) {
	var req struct {
		KeyID   string `json:"key_id"`
		ModelID string `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.KeyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_id is required"})
		return
	}

	key, ok := h.Store.GetServerAPIKeyByID(req.KeyID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}
	sv, ok := h.Store.GetServer(key.ServerID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	testModelID := req.ModelID
	if testModelID == "" {
		models := h.Store.GetServerModelsByServer(key.ServerID)
		if len(models) > 0 {
			testModelID = models[0].ModelID
		}
	}

	if h.Debug {
		log.Printf("[DEBUG] test-key: key=%s server=%s model=%s", req.KeyID, sv.Name, testModelID)
	}

	r := llm_storage.TestKeyWithModel(sv.APIURL, key.APIKey, sv.APIType, testModelID)
	result := TestResult{Status: r.Status, Duration: r.Duration, Error: r.Error, ModelID: r.ModelID}
	if h.Debug {
		log.Printf("[DEBUG] test-key: key=%s result=%s duration=%s", req.KeyID, result.Status, result.Duration)
	}
	h.Store.UpdateAPIKeyCheckResult(req.KeyID, result.Status, result.Duration)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) TestBatchKeys(c *gin.Context) {
	serverID := c.Query("server_id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server_id is required"})
		return
	}
	modelID := c.Query("model_id")

	sv, ok := h.Store.GetServer(serverID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	if h.Debug {
		log.Printf("[DEBUG] test-key batch: server=%s api_url=%s api_type=%s", sv.Name, sv.APIURL, sv.APIType)
	}

	keys := h.Store.GetServerAPIKeysByServer(serverID)
	pending := h.Store.GetPendingPoolKeys(serverID)
	seen := make(map[string]bool)
	var allKeys []llm_storage.ServerAPIKey
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

	models := h.Store.GetServerModelsByServer(serverID)
	var testModelID string
	if modelID != "" {
		testModelID = modelID
	} else if len(models) > 0 {
		testModelID = models[0].ModelID
	}

	testMu.Lock()
	testResults = nil
	testMu.Unlock()

	results := make([]TestResult, 0, len(allKeys))
	for _, key := range allKeys {
		if h.Debug {
			log.Printf("[DEBUG] test-key: key=%s model=%s", key.ID, testModelID)
		}
		time.Sleep(time.Duration(3+rand.Intn(13)) * time.Second)
		r := llm_storage.TestKeyWithModel(sv.APIURL, key.APIKey, sv.APIType, testModelID)
		result := TestResult{Status: r.Status, Duration: r.Duration, Error: r.Error, ModelID: r.ModelID, KeyID: key.ID, KeyMasked: utils.MaskKey(key.APIKey)}
		if h.Debug {
			log.Printf("[DEBUG] test-key: key=%s result=%s duration=%s", key.ID, result.Status, result.Duration)
		}
		results = append(results, result)
		h.Store.UpdateAPIKeyCheckResult(key.ID, result.Status, result.Duration)
	}

	testMu.Lock()
	testResults = results
	testMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *Handler) GetTestResults(c *gin.Context) {
	testMu.Lock()
	results := make([]TestResult, len(testResults))
	copy(results, testResults)
	testMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func TestSingleKeyExternal(apiURL, apiKey, apiType, modelID string) TestResult {
	r := llm_storage.TestKeyWithModel(apiURL, apiKey, apiType, modelID)
	return TestResult{
		Status:   r.Status,
		Duration: r.Duration,
		Error:    r.Error,
		ModelID:  r.ModelID,
	}
}
