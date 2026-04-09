package storage

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Storage struct {
	configPath string
	mu         sync.RWMutex
	rrCounters sync.Map
	config     *Config
}

type Config struct {
	Servers        []Server            `json:"servers" yaml:"servers"`
	ServerModels   []ServerModel       `json:"server_models" yaml:"server_models"`
	ServerAPIKeys  []ServerAPIKey      `json:"server_api_keys" yaml:"server_api_keys"`
	LocalModelMaps []LocalModelMapping `json:"local_model_maps" yaml:"local_model_maps"`
	Settings       Settings            `json:"settings" yaml:"settings"`
}

type Settings struct {
	Timeout int `json:"timeout" yaml:"timeout"`
}

type Server struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	APIURL  string `json:"api_url" yaml:"api_url"`
	APIType string `json:"api_type" yaml:"api_type"`
}

type ServerModel struct {
	ID        string `json:"id" yaml:"id"`
	ServerID  string `json:"server_id" yaml:"server_id"`
	ModelName string `json:"model_name" yaml:"model_name"`
	ModelID   string `json:"model_id" yaml:"model_id"`
}

type ServerAPIKey struct {
	ID       string `json:"id" yaml:"id"`
	ServerID string `json:"server_id" yaml:"server_id"`
	APIKey   string `json:"api_key" yaml:"api_key"`
	IsActive bool   `json:"is_active" yaml:"is_active"`
}

type LocalModelMapping struct {
	ID            string `json:"id" yaml:"id"`
	LocalModel    string `json:"local_model" yaml:"local_model"`
	ServerModelID string `json:"server_model_id" yaml:"server_model_id"`
}

func NewStorage(configPath string) (*Storage, error) {
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

func (s *Storage) GetServerAPIKeysByServer(serverID string) []ServerAPIKey {
	keys := s.GetServerAPIKeys()
	result := make([]ServerAPIKey, 0)
	for _, k := range keys {
		if k.ServerID == serverID && k.IsActive {
			result = append(result, k)
		}
	}
	return result
}

func (s *Storage) AddServerAPIKey(key ServerAPIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key.ID == "" {
		key.ID = uuid.New().String()
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
	if s.config.Settings.Timeout == 0 {
		return Settings{Timeout: 5}
	}
	return s.config.Settings
}

func (s *Storage) UpdateSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Settings = settings
	return s.save()
}