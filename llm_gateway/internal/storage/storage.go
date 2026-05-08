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

type Storage struct {
	configPath   string
	mu            sync.RWMutex
	rrCounters    sync.Map
	currentKeyIDs sync.Map
	config        *Config
}

type Config struct {
	Servers        []Server            `json:"servers" yaml:"servers"`
	ServerModels   []ServerModel       `json:"server_models" yaml:"server_models"`
	ServerAPIKeys  []ServerAPIKey      `json:"server_api_keys" yaml:"server_api_keys"`
	LocalModelMaps []LocalModelMapping `json:"local_model_maps" yaml:"local_model_maps"`
	Settings       Settings            `json:"settings" yaml:"settings"`
}

type Settings struct {
	Timeout               int  `json:"timeout" yaml:"timeout"`
	EnableNegativeWeight bool `json:"enable_negative_weight" yaml:"enable_negative_weight"`
	EnableRetry           bool `json:"enable_retry" yaml:"enable_retry"`
	WeightResetHours      int  `json:"weight_reset_hours" yaml:"weight_reset_hours"`
	Weight4xx             int  `json:"weight_4xx" yaml:"weight_4xx"`
	Weight5xx             int  `json:"weight_5xx" yaml:"weight_5xx"`
	MaxRetries            int  `json:"max_retries" yaml:"max_retries"`
	TimeoutWeight          int `json:"timeout_weight" yaml:"timeout_weight"`
	ConnectTimeoutWeight  int `json:"connect_timeout_weight" yaml:"connect_timeout_weight"`
	EnableRetryOnTimeout  bool `json:"enable_retry_on_timeout" yaml:"enable_retry_on_timeout"`
	ConnectTimeout int `json:"connect_timeout" yaml:"connect_timeout"`
	EnableAutoCheckAPIKey bool `json:"enable_auto_check_api_key" yaml:"enable_auto_check_api_key"`
	AutoCheckIntervalHours int `json:"auto_check_interval_hours" yaml:"auto_check_interval_hours"`
}

type Server struct {
	ID           string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	APIURL      string `json:"api_url" yaml:"api_url"`
	APIType     string `json:"api_type" yaml:"api_type"`
	Timeout     int    `json:"timeout" yaml:"timeout"` // 分鐘 (0 表示使用全局设置)
}

type ServerModel struct {
	ID        string `json:"id" yaml:"id"`
	ServerID  string `json:"server_id" yaml:"server_id"`
	ModelName string `json:"model_name" yaml:"model_name"`
	ModelID   string `json:"model_id" yaml:"model_id"`
}

type ServerAPIKey struct {
	ID              string    `json:"id" yaml:"id"`
	ServerID       string    `json:"server_id" yaml:"server_id"`
	APIKey         string    `json:"api_key" yaml:"api_key"`
	IsActive       bool      `json:"is_active" yaml:"is_active"`
	Status         string    `json:"status" yaml:"status"` // enabled, disabled, auto
	Notes          string    `json:"notes" yaml:"notes"`
	NegativeWeight int64     `json:"negative_weight" yaml:"negative_weight"`
	LastResetTime  time.Time `json:"last_reset_time" yaml:"last_reset_time"`
	LastCheckTime  time.Time `json:"last_check_time" yaml:"last_check_time"`
	LastCheckResult bool     `json:"last_check_result" yaml:"last_check_result"`
}

type LocalModelMapping struct {
	ID            string `json:"id" yaml:"id"`
	LocalModel    string `json:"local_model" yaml:"local_model"`
	ServerModelID string `json:"server_model_id" yaml:"server_model_id"`
}

func NewStorage(configPath string) (*Storage, error) {
	rand.Seed(time.Now().UnixNano())
	s := &Storage{configPath: configPath, config: &Config{}}

	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, s.config); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Storage) save() error {
	data, err := yaml.Marshal(s.config)
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0644)
}

func (s *Storage) GetServers() []Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Server, len(s.config.Servers))
	copy(result, s.config.Servers)
	return result
}

func (s *Storage) GetServer(id string) *Server {
	servers := s.GetServers()
	for i := range servers {
		if servers[i].ID == id {
			return &servers[i]
		}
	}
	return nil
}

func (s *Storage) AddServer(server Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if server.ID == "" {
		server.ID = uuid.New().String()
	}

	s.config.Servers = append(s.config.Servers, server)
	return s.save()
}

func (s *Storage) UpdateServer(server Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.Servers {
		if s.config.Servers[i].ID == server.ID {
			s.config.Servers[i] = server
			return s.save()
		}
	}
	return fmt.Errorf("server not found")
}

func (s *Storage) DeleteServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newServers := make([]Server, 0)
	for _, srv := range s.config.Servers {
		if srv.ID != id {
			newServers = append(newServers, srv)
		}
	}
	s.config.Servers = newServers

	newModels := make([]ServerModel, 0)
	for _, m := range s.config.ServerModels {
		if m.ServerID != id {
			newModels = append(newModels, m)
		}
	}
	s.config.ServerModels = newModels

	newKeys := make([]ServerAPIKey, 0)
	for _, k := range s.config.ServerAPIKeys {
		if k.ServerID != id {
			newKeys = append(newKeys, k)
		}
	}
	s.config.ServerAPIKeys = newKeys

	return s.save()
}

func (s *Storage) GetServerModels() []ServerModel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ServerModel, len(s.config.ServerModels))
	copy(result, s.config.ServerModels)
	return result
}

func (s *Storage) GetServerModel(id string) *ServerModel {
	models := s.GetServerModels()
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	return nil
}

func (s *Storage) GetServerModelsByServer(serverID string) []ServerModel {
	models := s.GetServerModels()
	result := make([]ServerModel, 0)
	for _, m := range models {
		if m.ServerID == serverID {
			result = append(result, m)
		}
	}
	return result
}

func (s *Storage) AddServerModel(model ServerModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if model.ID == "" {
		model.ID = uuid.New().String()
	}

	s.config.ServerModels = append(s.config.ServerModels, model)
	return s.save()
}

func (s *Storage) UpdateServerModel(model ServerModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.ServerModels {
		if s.config.ServerModels[i].ID == model.ID {
			s.config.ServerModels[i] = model
			return s.save()
		}
	}
	return fmt.Errorf("server model not found")
}

func (s *Storage) DeleteServerModel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newModels := make([]ServerModel, 0)
	for _, m := range s.config.ServerModels {
		if m.ID != id {
			newModels = append(newModels, m)
		}
	}
	s.config.ServerModels = newModels

	newMaps := make([]LocalModelMapping, 0)
	for _, m := range s.config.LocalModelMaps {
		if m.ServerModelID != id {
			newMaps = append(newMaps, m)
		}
	}
	s.config.LocalModelMaps = newMaps

	return s.save()
}

func (s *Storage) GetServerAPIKeys() []ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ServerAPIKey, len(s.config.ServerAPIKeys))
	copy(result, s.config.ServerAPIKeys)
	return result
}

func (s *Storage) GetServerAPIKey(id string) *ServerAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.config.ServerAPIKeys {
		if s.config.ServerAPIKeys[i].ID == id {
			return &s.config.ServerAPIKeys[i]
		}
	}
	return nil
}

func (s *Storage) GetServerAPIKeysByServer(serverID string) []ServerAPIKey {
	keys := s.GetServerAPIKeys()
	result := make([]ServerAPIKey, 0)
	for _, k := range keys {
		if k.ServerID != serverID || !k.IsActive {
			continue
		}
		if k.Status == "disabled" {
			continue
		}
		if k.Status == "auto" && k.LastCheckTime.IsZero() {
		}
		if k.Status == "auto" && !k.LastCheckTime.IsZero() && !k.LastCheckResult {
			continue
		}
		result = append(result, k)
	}
	return result
}

func (s *Storage) AddServerAPIKey(key ServerAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key.ID == "" {
		key.ID = uuid.New().String()
	}
	if key.Status == "" {
		key.Status = "enabled"
	}

	s.config.ServerAPIKeys = append(s.config.ServerAPIKeys, key)
	return s.save()
}

func (s *Storage) UpdateServerAPIKey(key ServerAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.ServerAPIKeys {
		if s.config.ServerAPIKeys[i].ID == key.ID {
			s.config.ServerAPIKeys[i] = key
			return s.save()
		}
	}
	return fmt.Errorf("api key not found")
}

func (s *Storage) DeleteServerAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newKeys := make([]ServerAPIKey, 0)
	for _, k := range s.config.ServerAPIKeys {
		if k.ID != id {
			newKeys = append(newKeys, k)
		}
	}
	s.config.ServerAPIKeys = newKeys
	return s.save()
}

func (s *Storage) GetNextAPIKey(serverID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	if len(keys) == 0 {
		return nil
	}

	settings := s.GetSettings()
	if settings.EnableNegativeWeight {
		return s.GetLowestWeightAPIKey(serverID)
	}

	var counter *atomic.Uint64
	val, ok := s.rrCounters.Load(serverID)
	if !ok {
		counter = &atomic.Uint64{}
		s.rrCounters.Store(serverID, counter)
	} else {
		counter = val.(*atomic.Uint64)
	}
	idx := counter.Add(1) - 1
	return &keys[idx%uint64(len(keys))]
}

func (s *Storage) GetLowestWeightAPIKey(serverID string) *ServerAPIKey {
	keys := s.GetServerAPIKeysByServer(serverID)
	if len(keys) == 0 {
		return nil
	}

	settings := s.GetSettings()
	resetDuration := time.Duration(settings.WeightResetHours) * time.Hour

	currentKeyID, hasCurrent := s.currentKeyIDs.Load(serverID)

	if hasCurrent && currentKeyID != "" {
		for _, k := range keys {
			if k.ID == currentKeyID {
				lowestWeight := k.NegativeWeight
				for _, ck := range keys {
					if ck.NegativeWeight < lowestWeight {
						lowestWeight = ck.NegativeWeight
					}
				}
				if k.NegativeWeight == lowestWeight {
					if k.LastResetTime.IsZero() || time.Since(k.LastResetTime) >= resetDuration {
						s.updateKeyResetTime(k.ID)
					}
					return &k
				}
				break
			}
		}
	}

	lowestWeight := keys[0].NegativeWeight
	for i := 1; i < len(keys); i++ {
		if keys[i].NegativeWeight < lowestWeight {
			lowestWeight = keys[i].NegativeWeight
		}
	}

	var bestKeys []ServerAPIKey
	for i := range keys {
		if keys[i].NegativeWeight == lowestWeight {
			bestKeys = append(bestKeys, keys[i])
		}
	}

	if len(bestKeys) == 1 {
		if bestKeys[0].LastResetTime.IsZero() || time.Since(bestKeys[0].LastResetTime) >= resetDuration {
			s.updateKeyResetTime(bestKeys[0].ID)
		}
		s.currentKeyIDs.Store(serverID, bestKeys[0].ID)
		return &bestKeys[0]
	}

	var expiredKeys []ServerAPIKey
	var activeKeys []ServerAPIKey

	for i := range bestKeys {
		if bestKeys[i].LastResetTime.IsZero() || time.Since(bestKeys[i].LastResetTime) >= resetDuration {
			expiredKeys = append(expiredKeys, bestKeys[i])
		} else {
			activeKeys = append(activeKeys, bestKeys[i])
		}
	}

	var candidates []ServerAPIKey
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

	var longestKeys []ServerAPIKey
	for i := range candidates {
		if candidates[i].LastResetTime.Equal(longestReset) {
			longestKeys = append(longestKeys, candidates[i])
		}
	}

	idx := rand.Intn(len(longestKeys))
	if longestKeys[idx].LastResetTime.IsZero() || time.Since(longestKeys[idx].LastResetTime) >= resetDuration {
		s.updateKeyResetTime(longestKeys[idx].ID)
	}
	s.currentKeyIDs.Store(serverID, longestKeys[idx].ID)
	return &longestKeys[idx]
}

func (s *Storage) updateKeyResetTime(keyID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.ServerAPIKeys {
		if s.config.ServerAPIKeys[i].ID == keyID {
			s.config.ServerAPIKeys[i].LastResetTime = time.Now()
			s.save()
			return
		}
	}
}

func (s *Storage) ClearCurrentKey(serverID string) {
	s.currentKeyIDs.Delete(serverID)
}

func (s *Storage) AddWeightToAPIKey(keyID string, weight int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.ServerAPIKeys {
		if s.config.ServerAPIKeys[i].ID == keyID {
			s.config.ServerAPIKeys[i].NegativeWeight += int64(weight)
			return s.save()
		}
	}
	return fmt.Errorf("api key not found")
}

func (s *Storage) ResetAllWeights(serverID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.ServerAPIKeys {
		if s.config.ServerAPIKeys[i].ServerID == serverID {
			s.config.ServerAPIKeys[i].NegativeWeight = 0
		}
	}
	return s.save()
}

func (s *Storage) ResetAllWeightsAllServers() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.ServerAPIKeys {
		s.config.ServerAPIKeys[i].NegativeWeight = 0
	}
	return s.save()
}

func (s *Storage) UpdateAPIKeyCheckResult(keyID string, result bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.ServerAPIKeys {
		if s.config.ServerAPIKeys[i].ID == keyID {
			s.config.ServerAPIKeys[i].LastCheckTime = time.Now()
			s.config.ServerAPIKeys[i].LastCheckResult = result
			return s.save()
		}
	}
	return fmt.Errorf("api key not found")
}

func (s *Storage) GetServersWithAPIKeys() map[string]struct {
	Server      *Server
	ServerAPIKey *ServerAPIKey
} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]struct {
		Server      *Server
		ServerAPIKey *ServerAPIKey
	})

	serverMap := make(map[string]*Server)
	for i := range s.config.Servers {
		serverMap[s.config.Servers[i].ID] = &s.config.Servers[i]
	}

	for i := range s.config.ServerAPIKeys {
		key := &s.config.ServerAPIKeys[i]
		if key.Status == "disabled" {
			continue
		}
		if key.Status == "auto" && key.LastCheckTime.IsZero() {
		}
		if key.Status == "auto" && !key.LastCheckTime.IsZero() && !key.LastCheckResult {
			continue
		}
		server, ok := serverMap[key.ServerID]
		if !ok {
			continue
		}
		result[key.ID] = struct {
			Server      *Server
			ServerAPIKey *ServerAPIKey
		}{
			Server:      server,
			ServerAPIKey: key,
		}
	}

	return result
}

func (s *Storage) GetLocalModelMaps() []LocalModelMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]LocalModelMapping, len(s.config.LocalModelMaps))
	copy(result, s.config.LocalModelMaps)
	return result
}

func (s *Storage) GetLocalModelMap(localModel string) *LocalModelMapping {
	maps := s.GetLocalModelMaps()
	for _, m := range maps {
		if m.LocalModel == localModel {
			return &m
		}
	}
	return nil
}

func (s *Storage) AddLocalModelMap(mapping LocalModelMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mapping.ID == "" {
		mapping.ID = uuid.New().String()
	}

	s.config.LocalModelMaps = append(s.config.LocalModelMaps, mapping)
	return s.save()
}

func (s *Storage) UpdateLocalModelMap(mapping LocalModelMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.LocalModelMaps {
		if s.config.LocalModelMaps[i].ID == mapping.ID {
			s.config.LocalModelMaps[i] = mapping
			return s.save()
		}
	}
	return fmt.Errorf("mapping not found")
}

func (s *Storage) DeleteLocalModelMap(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newMaps := make([]LocalModelMapping, 0)
	for _, m := range s.config.LocalModelMaps {
		if m.ID != id {
			newMaps = append(newMaps, m)
		}
	}
	s.config.LocalModelMaps = newMaps
	return s.save()
}

func (s *Storage) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings := s.config.Settings
	if settings.Timeout == 0 {
		settings.Timeout = 5
	}
	if settings.WeightResetHours == 0 {
		settings.WeightResetHours = 4
	}
	if settings.Weight4xx == 0 {
		settings.Weight4xx = 10
	}
	if settings.Weight5xx == 0 {
		settings.Weight5xx = 50
	}
	if settings.MaxRetries == 0 {
		settings.MaxRetries = 3
	}
	if settings.TimeoutWeight == 0 {
		settings.TimeoutWeight = 30
	}
	if settings.ConnectTimeoutWeight == 0 {
		settings.ConnectTimeoutWeight = 10
	}
	if settings.ConnectTimeout == 0 {
		settings.ConnectTimeout = 10
	}
	if settings.AutoCheckIntervalHours == 0 {
		settings.AutoCheckIntervalHours = 6
	}
	return settings
}

func (s *Storage) UpdateSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Settings = settings
	return s.save()
}