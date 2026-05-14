package api

import (
	"net/http"
	"sync"

	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
	"llm_gateway/internal/version"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Store *storage.Storage
	Debug bool
}

// TestResult tracks ephemeral API key test results.
type TestResult struct {
	KeyID     string `json:"key_id,omitempty"`
	KeyMasked string `json:"key_masked,omitempty"`
	Status    string `json:"status"`
	Duration  string `json:"duration"`
	Error     string `json:"error,omitempty"`
	ModelID   string `json:"model_id,omitempty"`
}

// --- Version ---

func (h *Handler) GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": version.Version, "build_date": version.BuildDate})
}

// --- Servers CRUD ---

func (h *Handler) ListServers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"servers": h.Store.GetServers()})
}

func (h *Handler) CreateServer(c *gin.Context) {
	var sv storage.Server
	if err := c.ShouldBindJSON(&sv); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.AddServer(sv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) UpdateServer(c *gin.Context) {
	var sv storage.Server
	if err := c.ShouldBindJSON(&sv); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.UpdateServer(sv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) DeleteServer(c *gin.Context) {
	if err := h.Store.DeleteServer(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Server Models CRUD ---

func (h *Handler) ListServerModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"server_models": h.Store.GetServerModels()})
}

func (h *Handler) CreateServerModel(c *gin.Context) {
	var m storage.ServerModel
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.AddServerModel(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) UpdateServerModel(c *gin.Context) {
	var m storage.ServerModel
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.UpdateServerModel(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) DeleteServerModel(c *gin.Context) {
	if err := h.Store.DeleteServerModel(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- API Keys CRUD ---

func (h *Handler) ListAPIKeys(c *gin.Context) {
	keys := h.Store.GetServerAPIKeys()
	masked := make([]storage.ServerAPIKey, len(keys))
	for i, k := range keys {
		masked[i] = k
		masked[i].APIKey = utils.MaskKey(k.APIKey)
	}
	c.JSON(http.StatusOK, gin.H{"server_api_keys": masked})
}

func (h *Handler) CreateAPIKey(c *gin.Context) {
	var k storage.ServerAPIKey
	if err := c.ShouldBindJSON(&k); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.AddServerAPIKey(k); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) UpdateAPIKey(c *gin.Context) {
	var k storage.ServerAPIKey
	if err := c.ShouldBindJSON(&k); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.UpdateServerAPIKey(k); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) DeleteAPIKey(c *gin.Context) {
	if err := h.Store.DeleteServerAPIKey(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Pending Pool ---

func (h *Handler) ListPendingPool(c *gin.Context) {
	servers := h.Store.GetServers()
	result := make(map[string][]storage.ServerAPIKey)
	for _, sv := range servers {
		keys := h.Store.GetPendingPoolKeys(sv.ID)
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
}

// --- Local Model Maps CRUD ---

func (h *Handler) ListLocalModelMaps(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"local_model_maps": h.Store.GetLocalModelMaps()})
}

func (h *Handler) CreateLocalModelMap(c *gin.Context) {
	var lm storage.LocalModelMapping
	if err := c.ShouldBindJSON(&lm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.AddLocalModelMap(lm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) UpdateLocalModelMap(c *gin.Context) {
	var lm storage.LocalModelMapping
	if err := c.ShouldBindJSON(&lm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Store.UpdateLocalModelMap(lm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) DeleteLocalModelMap(c *gin.Context) {
	if err := h.Store.DeleteLocalModelMap(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Settings ---

func (h *Handler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.Store.GetSettings())
}

func (h *Handler) UpdateSettings(c *gin.Context) {
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
	if err := h.Store.UpdateSettings(st); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Weight Reset ---

func (h *Handler) ResetAllWeightsAllServers(c *gin.Context) {
	if err := h.Store.ResetAllWeightsAllServers(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) ResetWeights(c *gin.Context) {
	if err := h.Store.ResetAllWeights(c.Param("serverId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Test Results ---

var (
	testResults []TestResult
	testMu      sync.Mutex
)
