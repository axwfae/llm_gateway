package storage

import (
	"fmt"
	"math/rand"
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
	APIKey            string    `json:"api_key" yaml:"api_key"`
	IsActive          bool      `json:"is_active" yaml:"is_active"`
	Status            string    `json:"status" yaml:"status"` // enabled, disabled, auto
	Notes             string    `json:"notes" yaml:"notes"`
	NegativeWeight    int       `json:"negative_weight" yaml:"-"` // runtime only, not persisted
	LastResetTime     time.Time `json:"last_reset_time" yaml:"-"` // runtime only
	LastCheckTime     time.Time `json:"last_check_time" yaml:"-"` // runtime only
	LastCheckResult   string    `json:"last_check_result" yaml:"-"` // runtime only
	LastCheckDuration string    `json:"last_check_duration" yaml:"-"` // runtime only
	CreatedAt         time.Time `json:"created_at" yaml:"created_at"`
	PendingPool       bool      `json:"pending_pool" yaml:"-"` // runtime only, computed
}

type LocalModelMapping struct {
	ID            string `json:"id" yaml:"id"`
	LocalModel    string `json:"local_model" yaml:"local_model"`
	ServerModelID string `json:"server_model_id" yaml:"server_model_id"`
}

// ---------------------------------------------------------------------------
// Runtime state (in-memory only, not persisted to YAML)
// ---------------------------------------------------------------------------

type runtimeState struct {
	mu               sync.RWMutex
	negativeWeights  map[string]int       // keyID → weight
	lastResetTimes   map[string]time.Time // keyID → last reset time
	lastCheckTimes   map[string]time.Time // keyID → last check time
	lastCheckResults map[string]string    // keyID → result
	lastCheckDur     map[string]string    // keyID → duration
	pendingPools     map[string]bool      // keyID → in pending pool
	lastUsedTimes    map[string]time.Time // keyID → last selected for use
}

func newRuntimeState(keys []ServerAPIKey) *runtimeState {
	rs := &runtimeState{
		negativeWeights:  make(map[string]int),
		lastResetTimes:   make(map[string]time.Time),
		lastCheckTimes:   make(map[string]time.Time),
		lastCheckResults: make(map[string]string),
		lastCheckDur:     make(map[string]string),
		pendingPools:     make(map[string]bool),
		lastUsedTimes:    make(map[string]time.Time),
	}
	for _, k := range keys {
		rs.negativeWeights[k.ID] = 0
		rs.lastResetTimes[k.ID] = k.LastResetTime
		rs.pendingPools[k.ID] = computePendingPool(k.Status, "")
	}
	return rs
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
	runtime      *runtimeState
}

func NewStorage(configPath string) (*Storage, error) {
	s := &Storage{
		configPath: configPath,
		config:     &Config{},
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
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
	s.runtime = newRuntimeState(s.config.ServerAPIKeys)
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

// save persists only config data (servers, models, keys metadata, mappings, settings).
// Runtime state (weights, check results) is intentionally NOT persisted.
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
// applyRuntime overlays runtime state onto a ServerAPIKey copy
// ---------------------------------------------------------------------------

func (s *Storage) applyRuntime(k ServerAPIKey) ServerAPIKey {
	s.runtime.mu.RLock()
	defer s.runtime.mu.RUnlock()
	if w, ok := s.runtime.negativeWeights[k.ID]; ok {
		k.NegativeWeight = w
	}
	if t, ok := s.runtime.lastResetTimes[k.ID]; ok {
		k.LastResetTime = t
	}
	if t, ok := s.runtime.lastCheckTimes[k.ID]; ok {
		k.LastCheckTime = t
	}
	if r, ok := s.runtime.lastCheckResults[k.ID]; ok {
		k.LastCheckResult = r
	}
	if d, ok := s.runtime.lastCheckDur[k.ID]; ok {
		k.LastCheckDuration = d
	}
	if p, ok := s.runtime.pendingPools[k.ID]; ok {
		k.PendingPool = p
	}
	return k
}

func (s *Storage) applyRuntimeAll(keys []ServerAPIKey) []ServerAPIKey {
	out := make([]ServerAPIKey, len(keys))
	for i, k := range keys {
		out[i] = s.applyRuntime(k)
	}
	return out
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

// GetServerAPIKeys returns keys with runtime state overlaid.
func (s *Storage) GetServerAPIKeys() []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ServerAPIKey, len(s.config.ServerAPIKeys))
	copy(out, s.config.ServerAPIKeys)
	return s.applyRuntimeAll(out)
}

// GetServerAPIKeysByServer returns enabled keys for a server with runtime state.
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
		// Apply runtime to check IsActive
		rk := s.applyRuntime(k)
		if !rk.IsActive {
			continue
		}
		if rk.Status == "auto" {
			if rk.LastCheckTime.IsZero() {
				continue
			}
			if rk.LastCheckResult != "ok" && rk.LastCheckResult != "success" {
				continue
			}
		}
		out = append(out, rk)
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
// ServerAPIKey CRUD (only metadata persisted)
// ---------------------------------------------------------------------------

func (s *Storage) AddServerAPIKey(k ServerAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k.ServerID == "" {
		return fmt.Errorf("server_id is required")
	}
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
	// Initialize runtime state
	s.runtime.mu.Lock()
	s.runtime.negativeWeights[k.ID] = 0
	s.runtime.lastResetTimes[k.ID] = k.LastResetTime
	s.runtime.pendingPools[k.ID] = computePendingPool(k.Status, "")
	s.runtime.mu.Unlock()

	s.config.ServerAPIKeys = append(s.config.ServerAPIKeys, k)
	return s.save()
}

// UpdateServerAPIKey allows updating Notes and Status.
// IsActive is auto-derived from Status (enabled/auto → active, disabled → inactive).
// ServerID and APIKey are immutable after creation.
func (s *Storage) UpdateServerAPIKey(k ServerAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.config.ServerAPIKeys {
		if existing.ID == k.ID {
			s.config.ServerAPIKeys[i].Notes = k.Notes
			s.config.ServerAPIKeys[i].Status = k.Status
			s.config.ServerAPIKeys[i].IsActive = k.Status != "disabled"

			var lastResult string
			s.runtime.mu.RLock()
			if r, ok := s.runtime.lastCheckResults[k.ID]; ok {
				lastResult = r
			}
			s.runtime.mu.RUnlock()
			s.runtime.mu.Lock()
			s.runtime.pendingPools[k.ID] = computePendingPool(k.Status, lastResult)
			s.runtime.mu.Unlock()

			return s.save()
		}
	}
	return fmt.Errorf("api key %s not found", k.ID)
}

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

	// Clean runtime state
	s.runtime.mu.Lock()
	delete(s.runtime.negativeWeights, id)
	delete(s.runtime.lastResetTimes, id)
	delete(s.runtime.lastCheckTimes, id)
	delete(s.runtime.lastCheckResults, id)
	delete(s.runtime.lastCheckDur, id)
	delete(s.runtime.pendingPools, id)
	delete(s.runtime.lastUsedTimes, id)
	s.runtime.mu.Unlock()

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

// GetNextAPIKey returns the next key for a server.
//   round-robin: cycles through keys on every call.
//   negative-weight: selects lowest-weight key and sticks with it until error.
func (s *Storage) GetNextAPIKey(serverID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	if len(keys) == 0 {
		if s.TestUntestedAutoKeys(serverID) {
			keys = s.GetServerAPIKeysByServer(serverID)
		}
		if len(keys) == 0 {
			return nil
		}
	}

	settings := s.GetSettings()
	if settings.EnableNegativeWeight {
		return s.getNextKeyNegativeWeight(serverID, keys)
	}

	// round-robin
	counterVal, _ := s.rrCounters.LoadOrStore(serverID, &atomic.Uint64{})
	counter := counterVal.(*atomic.Uint64)
	idx := counter.Add(1) - 1
	return &keys[idx%uint64(len(keys))]
}

// getNextKeyNegativeWeight implements sticky negative-weight selection.
// If there's already a current key for this server, reuse it (stick).
// Otherwise select the lowest-weight key and record it as current.
func (s *Storage) getNextKeyNegativeWeight(serverID string, keys []ServerAPIKey) *ServerAPIKey {
	// Sticky: reuse current key if still valid
	currentKeyID := s.GetCurrentKey(serverID)
	if currentKeyID != "" {
		for i := range keys {
			if keys[i].ID == currentKeyID {
				return &keys[i]
			}
		}
	}

	// No current key → select new one
	selected := s.getLowestWeightKey(keys)
	if selected != nil {
		s.runtime.mu.Lock()
		s.runtime.lastUsedTimes[selected.ID] = time.Now()
		s.runtime.mu.Unlock()
	}
	return selected
}

// getLowestWeightKey selects the key with the lowest negative weight.
// Tiebreakers in order:
//   1. lowest negative weight
//   2. oldest lastUsedTime (least recently used = most remaining timeout)
//   3. random
func (s *Storage) getLowestWeightKey(keys []ServerAPIKey) *ServerAPIKey {
	if len(keys) == 0 {
		return nil
	}

	lowest := keys[0].NegativeWeight
	for _, k := range keys[1:] {
		if k.NegativeWeight < lowest {
			lowest = k.NegativeWeight
		}
	}

	var best []ServerAPIKey
	for _, k := range keys {
		if k.NegativeWeight == lowest {
			best = append(best, k)
		}
	}

	if len(best) == 1 {
		return &best[0]
	}

	// Tiebreaker: pick the one with oldest lastUsedTime (least recently used)
	s.runtime.mu.RLock()
	oldest := best[0]
	oldestTime := s.runtime.lastUsedTimes[oldest.ID]
	for _, k := range best[1:] {
		t := s.runtime.lastUsedTimes[k.ID]
		if t.Before(oldestTime) {
			oldest = k
			oldestTime = t
		}
	}
	s.runtime.mu.RUnlock()

	// Check if all remaining candidates have the same lastUsedTime (incl. zero)
	s.runtime.mu.RLock()
	sameCount := 0
	for _, k := range best {
		if s.runtime.lastUsedTimes[k.ID] == oldestTime {
			sameCount++
		}
	}
	s.runtime.mu.RUnlock()

	if sameCount == 1 {
		return &oldest
	}

	// Final tiebreaker: random among those with same lastUsedTime
	var tied []ServerAPIKey
	s.runtime.mu.RLock()
	for _, k := range best {
		if s.runtime.lastUsedTimes[k.ID] == oldestTime {
			tied = append(tied, k)
		}
	}
	s.runtime.mu.RUnlock()

	return &tied[rand.Intn(len(tied))]
}

func (s *Storage) GetLowestWeightAPIKey(serverID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	return s.getLowestWeightKey(keys)
}

// AddWeightToAPIKey modifies runtime state only (not persisted).
func (s *Storage) AddWeightToAPIKey(keyID string, weight int) {
	s.runtime.mu.Lock()
	defer s.runtime.mu.Unlock()
	s.runtime.negativeWeights[keyID] += weight
}

func (s *Storage) ClearCurrentKey(serverID string) {
	s.currentKeyIDs.Delete(serverID)
}

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
func (s *Storage) GetNextKeyForServer(serverID, excludeKeyID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	if len(keys) == 0 {
		return nil
	}

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
		counterVal, _ := s.rrCounters.LoadOrStore(serverID+"_retry", &atomic.Uint64{})
		counter := counterVal.(*atomic.Uint64)
		idx := counter.Add(1) - 1
		return &candidates[idx%uint64(len(candidates))]
	}

	// Use shared tiebreaker logic (lastUsedTime → random)
	result := s.getLowestWeightKey(candidates)
	if result != nil {
		s.runtime.mu.Lock()
		s.runtime.lastUsedTimes[result.ID] = time.Now()
		s.runtime.mu.Unlock()
	}
	return result
}

// ---------------------------------------------------------------------------
// Weight reset (runtime only)
// ---------------------------------------------------------------------------

func (s *Storage) ResetAllWeights(serverID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	s.runtime.mu.Lock()
	defer s.runtime.mu.Unlock()
	for _, k := range s.config.ServerAPIKeys {
		if k.ServerID == serverID {
			s.runtime.negativeWeights[k.ID] = 0
			s.runtime.lastResetTimes[k.ID] = now
		}
	}
	return nil
}

func (s *Storage) ResetAllWeightsAllServers() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	s.runtime.mu.Lock()
	defer s.runtime.mu.Unlock()
	for _, k := range s.config.ServerAPIKeys {
		s.runtime.negativeWeights[k.ID] = 0
		s.runtime.lastResetTimes[k.ID] = now
	}
	return nil
}

// ---------------------------------------------------------------------------
// Health check updates (runtime only)
// ---------------------------------------------------------------------------

func (s *Storage) UpdateAPIKeyCheckResult(keyID, result, duration string) {
	s.runtime.mu.Lock()
	defer s.runtime.mu.Unlock()
	now := time.Now()
	s.runtime.lastCheckTimes[keyID] = now
	s.runtime.lastCheckResults[keyID] = result
	s.runtime.lastCheckDur[keyID] = duration
	if result != "ok" && result != "success" {
		s.runtime.negativeWeights[keyID] += 5
	} else {
		s.runtime.negativeWeights[keyID] = 0
	}
	s.runtime.pendingPools[keyID] = computePendingPool(
		s.getKeyStatusUnsafe(keyID),
		result,
	)
}

func (s *Storage) getKeyStatusUnsafe(keyID string) string {
	for _, k := range s.config.ServerAPIKeys {
		if k.ID == keyID {
			return k.Status
		}
	}
	return "disabled"
}

func (s *Storage) GetPendingPoolKeys(serverID string) []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.runtime.mu.RLock()
	defer s.runtime.mu.RUnlock()
	var out []ServerAPIKey
	for _, k := range s.config.ServerAPIKeys {
		if k.ServerID != serverID {
			continue
		}
		if inPool, ok := s.runtime.pendingPools[k.ID]; ok && inPool {
			out = append(out, s.applyRuntime(k))
		}
	}
	return out
}

func (s *Storage) GetAllPendingPoolKeys() []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.runtime.mu.RLock()
	defer s.runtime.mu.RUnlock()
	var out []ServerAPIKey
	for _, k := range s.config.ServerAPIKeys {
		if inPool, ok := s.runtime.pendingPools[k.ID]; ok && inPool {
			out = append(out, s.applyRuntime(k))
		}
	}
	return out
}

func (s *Storage) GetServerAPIKeyByID(id string) (*ServerAPIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.config.ServerAPIKeys {
		if k.ID == id {
			rk := s.applyRuntime(k)
			return &rk, true
		}
	}
	return nil, false
}

// GetServerInfo resolves a local model name into upstream target.
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

	s.SetCurrentKey(serverID, key.ID)
	return
}
