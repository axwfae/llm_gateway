# LLM Gateway

> 最新版本：v0.9.6 | 構建日期：2026-04-11

## 專案概述

LLM Gateway 是一個 LLM API 閘道服務，提供：
- **Web UI** - 設定管理介面 (中英雙語)
- **API** - LLM API 代理服務，支援 API Key 輪換

## 功能需求

1. **伺服器設置** - 管理 LLM 伺服器 (名稱、API URL、API 類型)
2. **伺服器模型設置** - 設定各伺服器的模型 (關聯伺服器)
3. **API Key 設置** - 管理 API Keys (預設啟用，僅支援新增/刪除)
4. **本地模型映射** - 將本地模型名稱映射到伺服器模型
5. **API Key 輪換** - 每次請求自動輪換 API Key (支持輪詢模式/負權重模式)
6. **錯誤重試** - API Key 失敗時自動切換重試 (最多 3 次)
7. **重啟系統** - 重新載入配置
8. **日誌記錄** - 記錄 API Key 輪換及上游伺服器錯誤 (預設開啟)
9. **Timeout 設定** - 可設定請求超時時間 (3-15分鐘)

## 技術架構

- **語言**: Go 1.21
- **Web 框架**: Gin
- **配置儲存**: 直接讀寫 YAML 檔案 (使用 gopkg.in/yaml.v3)
- **容器**: Docker/Podman

## 目錄結構

```
llm_gateway/
├── backups/                  # 版本備份
│   ├── v0.1.0/ ~ v0.9.0/
├── cmd/main.go              # 主程式入口
├── config/config.yaml       # 配置文件
├── internal/
│   ├── storage/storage.go  # 儲存層 (YAML 讀寫)
│   ├── utils/utils.go      # 工具函數
│   ├── version/version.go  # 版本資訊
│   └── webui/templates.go # Web UI 模板 (中英雙語)
├── Dockerfile
├── docker-compose.yml
├── build.sh
└── README.md
```

## API 接口

### Port (可配置，預設 18869)

| 接口 | 方法 | 功能 |
|------|------|------|
| `/health` | GET | 健康檢查 (返回版本號) |
| `/v1/models` | GET | 模型列表 (返回所有可用的本地模型) |
| `/v1/chat/completions` | POST | Chat Completions API |
| `/v1/*` | ANY | 通用代理 |

### Port (可配置，預設 18866)

| 路徑 | 功能 |
|------|------|
| `/` | 首頁 |
| `/servers` | 伺服器設置 |
| `/server-models` | 伺服器模型設置 |
| `/api-keys` | API Key 設置 |
| `/local-models` | 本地模型映射 |
| `/settings` | 系統設置 |

## 配置範例 (config/config.yaml)

```yaml
servers:
  - id: "xxx"
    name: "ollama_c"
    api_url: "https://ollama.com/v1"
    api_type: "openai"

server_models:
  - id: "xxx"
    server_id: "xxx"
    model_name: "minimax-m2.5:cloud"
    model_id: "minimax-m2.5:cloud"

server_api_keys:
  - id: "xxx"
    server_id: "xxx"
    api_key: "your-api-key-here"
    is_active: true
    negative_weight: 0

local_model_maps:
  - id: "xxx"
    local_model: "minimax-m2.5"
    server_model_id: "xxx"

settings:
  timeout: 5
  enable_negative_weight: false
  enable_retry: false
  weight_reset_hours: 4
  weight_4xx: 10
  weight_5xx: 50
  max_retries: 3
```

## 使用方式

### 建構

```bash
podman build -t llm_gateway:latest .
```

### 執行

```bash
podman run -p 18869:18869 -p 18866:18866 \
  -v ./config:/app/config \
  llm_gateway:latest
```

### 自定義端口

```bash
podman run -p 8080:8080 -p 9090:9090 \
  -v ./config:/app/config \
  llm_gateway:latest \
  -api-port 8080 -ui-port 9090
```

### API 測試

```bash
# 健康檢查
curl http://localhost:18869/health

# 模型列表
curl http://localhost:18869/v1/models

# Chat Completions
curl -X POST http://localhost:18869/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.5",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

### 查看日誌

```bash
# 所有日誌
podman logs -f llm_gateway

# API Key 輪換日誌
podman logs llm_gateway | grep "API_KEY_ROTATION"

# 上游伺服器錯誤日誌
podman logs llm_gateway | grep "UPSTREAM_ERROR"

# 重試日誌
podman logs llm_gateway | grep "RETRY"

# 權重重置日誌
podman logs llm_gateway | grep "WEIGHT_RESET"
```

## API Key 輪換邏輯

### 模式 1：輪詢模式 (預設)
- 每個 Server 有獨立的 atomic 計數器
- 公式：`index = (counter % len(keys))`
- 不同 Server 的 API Key 輪換完全獨立
- 日誌預設開啟，無法關閉

### 模式 2：負權重模式
- 啟用後，每次請求選擇負權重最低的 API Key
- 錯誤時增加權重：
  - 4xx 錯誤：+weight_4xx (預設 10)
  - 5xx 錯誤 / 網路錯誤：+weight_5xx (預設 50)
- 每 2-8 小時 (預設 4 小時) 將所有權重重置為 0

### 錯誤重試功能
- 啟用後，當 API Key 發生錯誤時，自動切換到下一個 Key 重試
- 最多重試次數可設定 (預設 3 次)
- 4xx 錯誤立即切換 Key
- 5xx 錯誤繼續重試直到成功或耗盡所有重試次數
- 未達最大重試次數的錯誤不會回傳給下層

## 版本記錄

| 版本 | 日期 | 說明 |
|------|------|------|
| v0.9.6 | 2026-04-11 | 負權重模式：優先選擇已過期的 key，實現真正独立重置时间 |
| v0.9.5 | 2026-04-11 | 負權重模式：每個 key 獨立重置时间，選擇時先比權重再比重置时间最後隨機 |
| v0.9.4 | 2026-04-11 | 負權重模式：保持同一 key 直到錯誤才重選，避免每次都重選 |
| v0.9.3 | 2026-04-11 | 新增 /v1/models 模型列表接口 |
| v0.9.2 | 2026-04-11 | 負權重重置週期預設值改為 4 小時 (範圍 2-8)、系統設置頁面說明更新為中英雙語 |
| v0.9.1 | 2026-04-11 | 負權重模式：最低權重 key 中隨機選擇，避免固定順序導致負載不均 |
| v0.9.0 | 2026-04-10 | 新增負權重模式、新增錯誤重試功能、API Key 顯示負權重值 |
| v0.8.0 | 2025-04-09 | 移除 LOG_API_KEY 環境變數、動態端口顯示、按鈕文字優化 |
| v0.7.0 | 2025-04-09 | 代碼優化：刪除未使用檔案、提取共用函數、修復錯誤處理 |
| v0.6.0 | 2025-04-09 | Timeout 設定功能、版本號顯示 |
| v0.5.0 | 2025-04-09 | Timeout 調整至 120秒 |
| v0.4.0 | 2025-04-09 | 新增上游伺服器錯誤日誌 |
| v0.3.0 | 2025-04-09 | 獨立 API Key 輪換計數器 |
| v0.2.0 | 2025-04-09 | 代碼優化、HTTP Client |
| v0.1.0 | 2025-04-09 | 初始版本 |

## 待改進事項

1. **安全驗證**
   - API URL SSRF 風險驗證
   - 請求大小限制

2. **功能擴展**
   - 支援更多 API 類型
   - API Key 映射支援
3. ** docker file **
   - https://hub.docker.com/r/wuyong1977/llm_gateway


<img width="1326" height="521" alt="截图_2026-04-09_10-23-51" src="https://github.com/user-attachments/assets/fac599bd-561d-4a44-934b-f714fdd0db28" />

<img width="1337" height="641" alt="截图_2026-04-09_10-24-10" src="https://github.com/user-attachments/assets/fdeda9d3-3ec3-4f75-afa5-728393dbf7bb" />

<img width="1518" height="668" alt="2026-04-11 17-01-54 的螢幕擷圖" src="https://github.com/user-attachments/assets/dddedb3c-4826-402c-84b7-e60833a8d523" />

<img width="1525" height="641" alt="2026-04-11 12-22-33 的螢幕擷圖" src="https://github.com/user-attachments/assets/9001b975-4afc-4935-a09b-b5984eb1808c" />

<img width="1525" height="692" alt="2026-04-11 12-22-55 的螢幕擷圖" src="https://github.com/user-attachments/assets/6a88c767-5fa6-45f4-af5f-09136560bc3e" />

<img width="920" height="199" alt="截图_2026-04-09_10-25-20" src="https://github.com/user-attachments/assets/9fab37c7-2142-4329-bbf7-2236bdc261d6" />

<img width="1154" height="538" alt="截图_2026-04-09_10-26-26" src="https://github.com/user-attachments/assets/c1d3dd1c-33a9-4b35-a69f-cdc504540f1d" />
