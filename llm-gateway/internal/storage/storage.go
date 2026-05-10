package storage

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Config & Settings
// ---------------------------------------------------------------------------

type Config struct {
	Servers        []Server            `json:"servers" yaml:"servers"`
	ServerModels   []ServerModel       `json:"server_models" yaml:"server_models"`
	ServerAPIKeys  []ServerAPIKey      `json:"server_api_keys" yaml:"server_api_keys"`
	LocalModelMaps []LocalModelMapping `json:"local_model_maps" yaml:"local_model_maps"`
	Settings       Settings            `json:"settings" yaml:"settings"`
}

type Settings struct {
	Timeout                int  `json:"timeout" yaml:"timeout"`                                   // global timeout in minutes
	EnableNegativeWeight   bool `json:"enable_negative_weight" yaml:"enable_negative_weight"`     // use negative-weight mode vs round-robin
	EnableRetry            bool `json:"enable_retry" yaml:"enable_retry"`                         // enable retry on failure
	WeightResetHours       int  `json:"weight_reset_hours" yaml:"weight_reset_hours"`             // hours between weight resets
	Weight4xx              int  `json:"weight_4xx" yaml:"weight_4xx"`                             // weight added on 4xx
	Weight5xx              int  `json:"weight_5xx" yaml:"weight_5xx"`                             // weight added on 5xx
	MaxRetries             int  `json:"max_retries" yaml:"max_retries"`                           // max retry count
	TimeoutWeight          int  `json:"timeout_weight" yaml:"timeout_weight"`                     // weight added on timeout
	ConnectTimeoutWeight   int  `json:"connect_timeout_weight" yaml:"connect_timeout_weight"`     // weight added on connect timeout
	EnableRetryOnTimeout   bool `json:"enable_retry_on_timeout" yaml:"enable_retry_on_timeout"`   // retry on timeout errors
	ConnectTimeout         int  `json:"connect_timeout" yaml:"connect_timeout"`                   // connect timeout in seconds
	EnableAutoCheckAPIKey  bool `json:"enable_auto_check_api_key" yaml:"enable_auto_check_api_key"`
	AutoCheckIntervalHours int  `json:"auto_check_interval_hours" yaml:"auto_check_interval_hours"`
}

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

type Server struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	APIURL  string `json:"api_url" yaml:"api_url"`
	APIType string `json:"api_type" yaml:"api_type"` // openai, anthropic, deepseek, ollama, other
	Timeout int    `json:"timeout" yaml:"timeout"`   // minutes, 0 = use global
}

type ServerModel struct {
	ID        string `json:"id" yaml:"id"`
	ServerID  string `json:"server_id" yaml:"server_id"`
	ModelName string `json:"model_name" yaml:"model_name"` // display name
	ModelID   string `json:"model_id" yaml:"model_id"`     // actual upstream model ID
}

type ServerAPIKey struct {
	ID                string    `json:"id" yaml:"id"`
	ServerID          string    `json:"server_id" yaml:"server_id"`
	APIKey            string    `json:"api_key" yaml:"api_key"`       // plain text (personal use)
	IsActive          bool      `json:"is_active" yaml:"is_active"`   // in rotation pool
	Status            string    `json:"status" yaml:"status"`         // enabled, disabled, auto
	Notes             string    `json:"notes" yaml:"notes"`
	NegativeWeight    int       `json:"negative_weight" yaml:"negative_weight"`
	LastResetTime     time.Time `json:"last_reset_time" yaml:"last_reset_time"`
	LastCheckTime     time.Time `json:"last_check_time" yaml:"last_check_time"`
	LastCheckResult   string    `json:"last_check_result" yaml:"last_check_result"`
	LastCheckDuration string    `json:"last_check_duration" yaml:"last_check_duration"`
	CreatedAt         time.Time `json:"created_at" yaml:"created_at"`
	// PendingPool feature: mark a key as candidate for the pending-pool.
	// When true, this key is included in "待選池" so it can be tested before
	// being fully enabled without affecting production traffic.
	PendingPool bool `json:"pending_pool" yaml:"pending_pool"`
}

type LocalModelMapping struct {
	ID            string `json:"id" yaml:"id"`
	LocalModel    string `json:"local_model" yaml:"local_model"`       // client-facing model name
	ServerModelID string `json:"server_model_id" yaml:"server_model_id"` // points to a ServerModel
}

// ---------------------------------------------------------------------------
// Storage
// ---------------------------------------------------------------------------

type Storage struct {
	configPath   string
	mu           sync.RWMutex
	rrCounters   sync.Map // map[serverID]*atomic.Uint64
	currentKeyIDs sync.Map // map[serverID]*atomic.Value (holds current key ID string)
	config       *Config
}

func NewStorage(configPath string) (*Storage, error) {
	s := &Storage{
		configPath: configPath,
		config:     &Config{},
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// create default config
			s.config = defaultConfig()
			if err := s.save(); err != nil {
				return nil, fmt.Errorf("create default config: %w", err)
			}
			return s, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, s.config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return s, nil
}

func defaultConfig() *Config {
	return &Config{
		Settings: Settings{
			Timeout:                5,
			EnableNegativeWeight:   true,
			EnableRetry:            true,
			WeightResetHours:       4,
			Weight4xx:              5,
			Weight5xx:              10,
			MaxRetries:             3,
			TimeoutWeight:          8,
			ConnectTimeoutWeight:   12,
			EnableRetryOnTimeout:   false,
			ConnectTimeout:         30,
			EnableAutoCheckAPIKey:  true,
			AutoCheckIntervalHours: 6,
		},
	}
}

func (s *Storage) save() error {
	data, err := yaml.Marshal(s.config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(s.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Getters (thread-safe reads)
// ---------------------------------------------------------------------------

func (s *Storage) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Settings
}

func (s *Storage) GetServers() []Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Server, len(s.config.Servers))
	copy(out, s.config.Servers)
	return out
}

func (s *Storage) GetServer(id string) (Server, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sv := range s.config.Servers {
		if sv.ID == id {
			return sv, true
		}
	}
	return Server{}, false
}

func (s *Storage) GetServerModels() []ServerModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ServerModel, len(s.config.ServerModels))
	copy(out, s.config.ServerModels)
	return out
}

func (s *Storage) GetServerModel(id string) (ServerModel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.config.ServerModels {
		if m.ID == id {
			return m, true
		}
	}
	return ServerModel{}, false
}

func (s *Storage) GetServerModelsByServer(serverID string) []ServerModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ServerModel
	for _, m := range s.config.ServerModels {
		if m.ServerID == serverID {
			out = append(out, m)
		}
	}
	return out
}

func (s *Storage) GetServerAPIKeys() []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ServerAPIKey, len(s.config.ServerAPIKeys))
	copy(out, s.config.ServerAPIKeys)
	return out
}

// GetServerAPIKeysByServer returns enabled keys for a server.
// If status is "auto" and LastCheckTime is zero, the key is excluded (not checked yet).
func (s *Storage) GetServerAPIKeysByServer(serverID string) []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ServerAPIKey
	for _, k := range s.config.ServerAPIKeys {
		if k.ServerID != serverID {
			continue
		}
		if k.Status == "disabled" {
			continue
		}
		if !k.IsActive {
			continue
		}
		if k.Status == "auto" && k.LastCheckTime.IsZero() {
			continue
		}
		out = append(out, k)
	}
	return out
}

func (s *Storage) GetLocalModelMaps() []LocalModelMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LocalModelMapping, len(s.config.LocalModelMaps))
	copy(out, s.config.LocalModelMaps)
	return out
}

func (s *Storage) GetLocalModelMap(localModel string) (LocalModelMapping, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.config.LocalModelMaps {
		if m.LocalModel == localModel {
			return m, true
		}
	}
	return LocalModelMapping{}, false
}

// ---------------------------------------------------------------------------
// Server CRUD
// ---------------------------------------------------------------------------

func (s *Storage) AddServer(sv Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv.ID = uuid.New().String()
	s.config.Servers = append(s.config.Servers, sv)
	return s.save()
}

func (s *Storage) UpdateServer(sv Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.config.Servers {
		if existing.ID == sv.ID {
			s.config.Servers[i] = sv
			return s.save()
		}
	}
	return fmt.Errorf("server %s not found", sv.ID)
}

func (s *Storage) DeleteServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, sv := range s.config.Servers {
		if sv.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("server %s not found", id)
	}
	s.config.Servers = append(s.config.Servers[:idx], s.config.Servers[idx+1:]...)
	// cascade delete server models, api keys
	var keptModels []ServerModel
	for _, m := range s.config.ServerModels {
		if m.ServerID != id {
			keptModels = append(keptModels, m)
		}
	}
	s.config.ServerModels = keptModels

	var keptKeys []ServerAPIKey
	for _, k := range s.config.ServerAPIKeys {
		if k.ServerID != id {
			keptKeys = append(keptKeys, k)
		}
	}
	s.config.ServerAPIKeys = keptKeys
	return s.save()
}

// ---------------------------------------------------------------------------
// ServerModel CRUD
// ---------------------------------------------------------------------------

func (s *Storage) AddServerModel(m ServerModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ServerID == "" {
		return fmt.Errorf("server_id is required")
	}
	// verify server exists
	found := false
	for _, sv := range s.config.Servers {
		if sv.ID == m.ServerID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("server %s not found", m.ServerID)
	}
	m.ID = uuid.New().String()
	s.config.ServerModels = append(s.config.ServerModels, m)
	return s.save()
}

func (s *Storage) UpdateServerModel(m ServerModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.config.ServerModels {
		if existing.ID == m.ID {
			s.config.ServerModels[i] = m
			return s.save()
		}
	}
	return fmt.Errorf("server model %s not found", m.ID)
}

func (s *Storage) DeleteServerModel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, m := range s.config.ServerModels {
		if m.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("server model %s not found", id)
	}
	s.config.ServerModels = append(s.config.ServerModels[:idx], s.config.ServerModels[idx+1:]...)
	// cascade delete local model mappings
	var keptMaps []LocalModelMapping
	for _, lm := range s.config.LocalModelMaps {
		if lm.ServerModelID != id {
			keptMaps = append(keptMaps, lm)
		}
	}
	s.config.LocalModelMaps = keptMaps
	return s.save()
}

// ---------------------------------------------------------------------------
// ServerAPIKey CRUD
// ---------------------------------------------------------------------------

func (s *Storage) AddServerAPIKey(k ServerAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k.ServerID == "" {
		return fmt.Errorf("server_id is required")
	}
	// verify server exists
	found := false
	for _, sv := range s.config.Servers {
		if sv.ID == k.ServerID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("server %s not found", k.ServerID)
	}
	k.ID = uuid.New().String()
	if k.Status == "" {
		k.Status = "enabled"
	}
	k.IsActive = true
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now()
	}
	k.PendingPool = computePendingPool(k.Status, "")
	s.config.ServerAPIKeys = append(s.config.ServerAPIKeys, k)
	return s.save()
}

// UpdateServerAPIKey only allows updating Notes and Status.
// ServerID and APIKey are immutable after creation.
func (s *Storage) UpdateServerAPIKey(k ServerAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.config.ServerAPIKeys {
		if existing.ID == k.ID {
			s.config.ServerAPIKeys[i].Notes = k.Notes
			s.config.ServerAPIKeys[i].Status = k.Status
			s.config.ServerAPIKeys[i].PendingPool = computePendingPool(k.Status, existing.LastCheckResult)
			return s.save()
		}
	}
	return fmt.Errorf("api key %s not found", k.ID)
}

// computePendingPool decides the automatic PendingPool flag:
//   enabled  → true (always in pool for testing)
//   disabled → false (never in pool)
//   auto     → true if last check is empty or ok/success, false if failed
func computePendingPool(status string, lastCheckResult string) bool {
	switch status {
	case "enabled":
		return true
	case "disabled":
		return false
	case "auto":
		if lastCheckResult == "" || lastCheckResult == "ok" || lastCheckResult == "success" {
			return true
		}
		return false
	default:
		return false
	}
}

func (s *Storage) DeleteServerAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, k := range s.config.ServerAPIKeys {
		if k.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("api key %s not found", id)
	}
	s.config.ServerAPIKeys = append(s.config.ServerAPIKeys[:idx], s.config.ServerAPIKeys[idx+1:]...)
	return s.save()
}

// ---------------------------------------------------------------------------
// LocalModelMapping CRUD
// ---------------------------------------------------------------------------

func (s *Storage) AddLocalModelMap(lm LocalModelMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lm.ID = uuid.New().String()
	s.config.LocalModelMaps = append(s.config.LocalModelMaps, lm)
	return s.save()
}

func (s *Storage) UpdateLocalModelMap(lm LocalModelMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.config.LocalModelMaps {
		if existing.ID == lm.ID {
			s.config.LocalModelMaps[i] = lm
			return s.save()
		}
	}
	return fmt.Errorf("local model map %s not found", lm.ID)
}

func (s *Storage) DeleteLocalModelMap(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, lm := range s.config.LocalModelMaps {
		if lm.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("local model map %s not found", id)
	}
	s.config.LocalModelMaps = append(s.config.LocalModelMaps[:idx], s.config.LocalModelMaps[idx+1:]...)
	return s.save()
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

func (s *Storage) UpdateSettings(st Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Settings = st
	return s.save()
}

// ---------------------------------------------------------------------------
// API Key selection (round-robin / negative-weight)
// ---------------------------------------------------------------------------

// GetNextAPIKey returns the next key for a server in round-robin mode.
// Only includes enabled + active keys (auto keys that have been checked).
func (s *Storage) GetNextAPIKey(serverID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	if len(keys) == 0 {
		return nil
	}

	settings := s.GetSettings()
	if settings.EnableNegativeWeight {
		return s.getLowestWeightKey(keys)
	}

	// round-robin
	counterVal, _ := s.rrCounters.LoadOrStore(serverID, &atomic.Uint64{})
	counter := counterVal.(*atomic.Uint64)
	idx := counter.Add(1) - 1
	return &keys[idx%uint64(len(keys))]
}

// getLowestWeightKey selects the key with the lowest negative weight.
// Among keys with equal lowest weight, prefers those past their reset window,
// then those with the oldest reset time.
func (s *Storage) getLowestWeightKey(keys []ServerAPIKey) *ServerAPIKey {
	if len(keys) == 0 {
		return nil
	}

	settings := s.GetSettings()
	now := time.Now()

	// find lowest weight
	lowest := keys[0].NegativeWeight
	for _, k := range keys[1:] {
		if k.NegativeWeight < lowest {
			lowest = k.NegativeWeight
		}
	}

	// collect keys with lowest weight
	var best []ServerAPIKey
	for _, k := range keys {
		if k.NegativeWeight == lowest {
			best = append(best, k)
		}
	}

	if len(best) == 1 {
		return &best[0]
	}

	// preference: key whose reset time has passed
	resetHours := time.Duration(settings.WeightResetHours) * time.Hour
	var expired []ServerAPIKey
	var notExpired []ServerAPIKey
	for _, k := range best {
		if k.LastResetTime.IsZero() || now.Sub(k.LastResetTime) >= resetHours {
			expired = append(expired, k)
		} else {
			notExpired = append(notExpired, k)
		}
	}

	var candidates []ServerAPIKey
	if len(expired) > 0 {
		candidates = expired
	} else {
		candidates = notExpired
	}

	if len(candidates) == 1 {
		return &candidates[0]
	}

	// pick the one with oldest last reset time
	oldest := candidates[0]
	for _, k := range candidates[1:] {
		if k.LastResetTime.Before(oldest.LastResetTime) {
			oldest = k
		}
	}
	return &oldest
}

// GetLowestWeightAPIKey returns the lowest-weight key for a server (used by UI/reset)
func (s *Storage) GetLowestWeightAPIKey(serverID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	return s.getLowestWeightKey(keys)
}

// AddWeightToAPIKey adds negative weight to a key and clears current-key record.
func (s *Storage) AddWeightToAPIKey(keyID string, weight int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, k := range s.config.ServerAPIKeys {
		if k.ID == keyID {
			s.config.ServerAPIKeys[i].NegativeWeight += weight
			_ = s.save()
			return
		}
	}
}

func (s *Storage) ClearCurrentKey(serverID string) {
	s.currentKeyIDs.Delete(serverID)
}

// SetCurrentKey stores the current key ID for a server (for retry exclusion).
func (s *Storage) SetCurrentKey(serverID, keyID string) {
	val, _ := s.currentKeyIDs.LoadOrStore(serverID, &atomic.Value{})
	v := val.(*atomic.Value)
	v.Store(keyID)
}

func (s *Storage) GetCurrentKey(serverID string) string {
	val, ok := s.currentKeyIDs.Load(serverID)
	if !ok {
		return ""
	}
	v := val.(*atomic.Value)
	keyID, _ := v.Load().(string)
	return keyID
}

// GetNextKeyForServer returns the next key excluding the current one (for retry).
// Uses negative-weight logic for selection.
func (s *Storage) GetNextKeyForServer(serverID, excludeKeyID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	if len(keys) == 0 {
		return nil
	}

	// filter out current failing key
	var candidates []ServerAPIKey
	for _, k := range keys {
		if k.ID != excludeKeyID {
			candidates = append(candidates, k)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	settings := s.GetSettings()
	if !settings.EnableNegativeWeight {
		// round-robin among candidates
		counterVal, _ := s.rrCounters.LoadOrStore(serverID+"_retry", &atomic.Uint64{})
		counter := counterVal.(*atomic.Uint64)
		idx := counter.Add(1) - 1
		return &candidates[idx%uint64(len(candidates))]
	}

	// negative-weight: find lowest
	now := time.Now()
	resetHours := time.Duration(settings.WeightResetHours) * time.Hour

	lowest := candidates[0].NegativeWeight
	for _, k := range candidates[1:] {
		if k.NegativeWeight < lowest {
			lowest = k.NegativeWeight
		}
	}

	var best []ServerAPIKey
	for _, k := range candidates {
		if k.NegativeWeight == lowest {
			best = append(best, k)
		}
	}
	if len(best) == 1 {
		return &best[0]
	}

	var expired []ServerAPIKey
	var notExpired []ServerAPIKey
	for _, k := range best {
		if k.LastResetTime.IsZero() || now.Sub(k.LastResetTime) >= resetHours {
			expired = append(expired, k)
		} else {
			notExpired = append(notExpired, k)
		}
	}

	pool := expired
	if len(pool) == 0 {
		pool = notExpired
	}

	oldest := pool[0]
	for _, k := range pool[1:] {
		if k.LastResetTime.Before(oldest.LastResetTime) {
			oldest = k
		}
	}
	return &oldest
}

// ---------------------------------------------------------------------------
// Weight reset
// ---------------------------------------------------------------------------

func (s *Storage) ResetAllWeights(serverID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i, k := range s.config.ServerAPIKeys {
		if k.ServerID == serverID {
			s.config.ServerAPIKeys[i].NegativeWeight = 0
			s.config.ServerAPIKeys[i].LastResetTime = now
		}
	}
	return s.save()
}

func (s *Storage) ResetAllWeightsAllServers() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i := range s.config.ServerAPIKeys {
		s.config.ServerAPIKeys[i].NegativeWeight = 0
		s.config.ServerAPIKeys[i].LastResetTime = now
	}
	return s.save()
}

// ---------------------------------------------------------------------------
// Health check updates
// ---------------------------------------------------------------------------

func (s *Storage) UpdateAPIKeyCheckResult(keyID, result, duration string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i, k := range s.config.ServerAPIKeys {
		if k.ID == keyID {
			s.config.ServerAPIKeys[i].LastCheckTime = now
			s.config.ServerAPIKeys[i].LastCheckResult = result
			s.config.ServerAPIKeys[i].LastCheckDuration = duration
			if result != "ok" && result != "success" {
				s.config.ServerAPIKeys[i].NegativeWeight += 5
			} else {
				s.config.ServerAPIKeys[i].NegativeWeight = 0
			}
			s.config.ServerAPIKeys[i].PendingPool = computePendingPool(k.Status, result)
			_ = s.save()
			return
		}
	}
}

// GetPendingPoolKeys returns keys with PendingPool=true for a server.
func (s *Storage) GetPendingPoolKeys(serverID string) []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ServerAPIKey
	for _, k := range s.config.ServerAPIKeys {
		if k.ServerID == serverID && k.PendingPool {
			out = append(out, k)
		}
	}
	return out
}

// GetAllPendingPoolKeys returns all keys with PendingPool=true.
func (s *Storage) GetAllPendingPoolKeys() []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ServerAPIKey
	for _, k := range s.config.ServerAPIKeys {
		if k.PendingPool {
			out = append(out, k)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Helpers for retry
// ---------------------------------------------------------------------------

// GetServerAPIKeyByID returns a key by its ID (unlocked, full key visible).
func (s *Storage) GetServerAPIKeyByID(id string) (*ServerAPIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.config.ServerAPIKeys {
		if k.ID == id {
			cp := k
			return &cp, true
		}
	}
	return nil, false
}

// GetServerInfo resolves a local model name into (server, serverModel, apiKey) for proxying.
// Returns the resolved model ID (what to send upstream), the target URL, the API key, and the API type.
func (s *Storage) GetServerInfo(localModel string) (modelID, targetURL, apiKey, apiType, serverID string, err error) {
	mapping, ok := s.GetLocalModelMap(localModel)
	if !ok {
		err = fmt.Errorf("local model '%s' not mapped", localModel)
		return
	}

	svModel, ok := s.GetServerModel(mapping.ServerModelID)
	if !ok {
		err = fmt.Errorf("server model '%s' not found", mapping.ServerModelID)
		return
	}

	sv, ok := s.GetServer(svModel.ServerID)
	if !ok {
		err = fmt.Errorf("server '%s' not found", svModel.ServerID)
		return
	}

	key := s.GetNextAPIKey(sv.ID)
	if key == nil {
		err = fmt.Errorf("no available API key for server '%s'", sv.ID)
		return
	}

	modelID = svModel.ModelID
	targetURL = sv.APIURL
	apiKey = key.APIKey
	apiType = sv.APIType
	serverID = sv.ID

	// Record current key for retry exclusion
	s.SetCurrentKey(serverID, key.ID)
	return
}