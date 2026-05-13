package storage

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

var testQuestions = []string{
	"What is 1+1?", "What is 2+2?", "What is 1+2?", "What is 3+1?",
	"What is 2+3?", "What is 1+3?", "What is 4+1?", "What is 5+1?",
	"What is 2+1?", "What is 1+4?", "What is 3+2?", "What is 1+5?",
	"What is 6+1?", "What is 7+1?", "What is 8+1?", "What is 1+6?",
	"What is 1+7?", "What is 1+8?", "What is 9+1?", "What is 10+1?",
}

type KeyTestResult struct {
	Status   string
	Duration string
	Error    string
	ModelID  string
}

// TestKeyWithModel sends a test request to an upstream LLM endpoint.
// It returns a result indicating success (ok) or failure (fail).
func TestKeyWithModel(apiURL, apiKey, apiType, modelID string) KeyTestResult {
	if modelID == "" {
		modelID = "gpt-3.5-turbo"
	}

	question := testQuestions[rand.Intn(len(testQuestions))]
	payload := map[string]interface{}{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "user", "content": question},
		},
		"max_tokens": 10,
	}

	body, _ := json.Marshal(payload)

	chatPath := "/chat/completions"
	switch apiType {
	case "anthropic":
		chatPath = "/v1/messages"
	case "ollama":
		chatPath = "/api/chat"
	default:
		if !bytes.Contains([]byte(apiURL), []byte("/v1")) {
			chatPath = "/v1/chat/completions"
		}
	}

	req, _ := http.NewRequest("POST", apiURL+chatPath, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if apiType == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	start := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return KeyTestResult{
			Status:   "fail",
			Duration: duration.String(),
			Error:    err.Error(),
			ModelID:  modelID,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return KeyTestResult{
			Status:   "ok",
			Duration: duration.String(),
			ModelID:  modelID,
		}
	}

	return KeyTestResult{
		Status:   "fail",
		Duration: duration.String(),
		Error:    "HTTP " + resp.Status,
		ModelID:  modelID,
	}
}

// TestUntestedAutoKeys finds auto-status keys that have never been tested,
// tests them immediately, and updates their check results.
// Returns true if any key was tested.
func (s *Storage) TestUntestedAutoKeys(serverID string) bool {
	s.mu.RLock()
	var apiURL, apiType string
	for _, sv := range s.config.Servers {
		if sv.ID == serverID {
			apiURL = sv.APIURL
			apiType = sv.APIType
			break
		}
	}
	var testModelID string
	for _, m := range s.config.ServerModels {
		if m.ServerID == serverID {
			testModelID = m.ModelID
			break
		}
	}
	type pendingTest struct {
		id     string
		apiKey string
	}
	var untested []pendingTest
	for _, k := range s.config.ServerAPIKeys {
		if k.ServerID != serverID {
			continue
		}
		if k.Status != "auto" {
			continue
		}
		rk := s.applyRuntime(k)
		if !rk.LastCheckTime.IsZero() {
			continue
		}
		untested = append(untested, pendingTest{id: k.ID, apiKey: k.APIKey})
	}
	s.mu.RUnlock()

	if len(untested) == 0 {
		return false
	}

	for _, pt := range untested {
		result := TestKeyWithModel(apiURL, pt.apiKey, apiType, testModelID)
		s.UpdateAPIKeyCheckResult(pt.id, result.Status, result.Duration)
	}
	return true
}
