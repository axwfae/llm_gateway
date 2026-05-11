# LLM Gateway

輕量級 LLM API 代理閘道服務，支援多後端路由、API Key 輪換、錯誤重試與 Web 管理介面。

## 工作原理

```
客戶端 (OpenAI-compatible SDK)
       │
       │ POST /v1/chat/completions {model: "llm_new", ...}
       ▼
┌─────────────────────────────────┐
│     API Proxy (Port 18869)      │
│                                 │
│  1. 解析請求中的 model 名稱      │
│  2. 查找本地模型映射             │
│     (local model → server model) │
│  3. 取得對應的服務器與實際模型    │
│  4. 從輪換池選取一個 API Key     │
│  5. 根據 API Type 決定路徑       │
│     (openai → /v1/chat/completions  │
│      anthropic → /v1/messages     │
│      ollama → /api/chat)          │
│  6. 轉發請求到上游 LLM 服務器    │
│  7. 失敗時自動重試/換 Key       │
└─────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│  上游 LLM 服務器                │
│  (OpenAI / Anthropic / DeepSeek │
│   / Ollama / NVIDIA / 其他)     │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│  Web UI (Port 18866)            │
│  管理介面：                      │
│  ├ 服務器 Servers               │
│  ├ 模型 Models                  │
│  ├ API Key                      │
│  ├ 待選池 Pending Pool          │
│  ├ 映射 Mapping                 │
│  └ 系統設定 Settings            │
└─────────────────────────────────┘
```

### 核心功能

| 功能 | 說明 |
|------|------|
| **多後端路由** | 透過「本地模型名稱 → 服務器模型」映射，一個端點路由到不同 LLM 後端 |
| **API Key 輪換** | 支援 Round-Robin 和負權重模式。後者會根據錯誤次數自動降低 Key 的優先級 |
| **錯誤重試** | 網路錯誤/超時/4xx/5xx 自動重試（可設定重試次數），每次更換 Key |
| **分離式超時** | TCP 連接超時和響應超時分開設定，避免慢連接佔用過久 |
| **自動健康檢查** | 定期測試 `auto` 模式的 Key，失敗的 Key 自動排除出輪換池 |
| **待選池** | 根據 Key 的工作模式自動管理。`開啟` 自動加入、`關閉` 排除、`自動` 通過測試後加入 |
| **中英雙語 UI** | 所有頁面同時顯示中文和英文標籤 |
| **連線池共享** | 所有 HTTP client 共用連線池，減少 TCP 連線建立開銷 |

### API 端點

| 路徑 | Port | 功能 |
|------|------|------|
| `/v1/chat/completions` | 18869 | OpenAI-compatible 聊天補全 |
| `/v1/models` | 18869 | 列出可用模型 |
| `/v1/` | 18869 | 通用代理（直接轉發） |
| `/health` | 18869 | 健康檢查 |

### 請求處理流程

```
/v1/chat/completions POST {model: "llm_new"}
  → 提取 model 字段
  → 查找 LocalModelMap("llm_new") → 得到 ServerModelID
  → 查找 ServerModel(id) → 得到 ServerID + 實際 ModelID
  → 查找 Server(id) → 得到 API URL + API Type
  → GetNextAPIKey(serverID) → 輪詢或取最低權重 Key
  → 替換 model 為 ModelID
  → 根據 API Type 決定 upstream 路徑：
      openai   → {api_url}/chat/completions
                 (若 api_url 不含 /v1 則自動補上)
      anthropic → {api_url}/v1/messages
      ollama   → {api_url}/api/chat
  → 發送請求到上游
  → 成功 → 直接回傳上游響應
  → 失敗 → 加權重 → 換 Key → 重試（最多 MaxRetries 次）
```

## 快速開始

### 1. 設定服務器

1. 開啟 Web UI `http://<NAS_IP>:18866`
2. 進入「服務器 Servers」頁面，新增 LLM 後端
3. 支援類型：`openai`、`anthropic`、`ollama`

### 2. 設定模型

3. 進入「模型 Models」頁面，為每個服務器新增支援的模型

### 3. 加入 API Key

4. 進入「API Keys」頁面，新增 API Key
   - **開啟 (Enabled)**: 直接加入待選池和輪換池
   - **關閉 (Disabled)**: 完全排除
   - **自動 (Auto)**: 通過健康檢查後才加入（失敗時自動排除）

### 4. 建立模型映射

5. 進入「映射 Mapping」頁面，建立「本地模型名稱 → 服務器模型」的映射

### 5. 開始使用

6. 使用 OpenAI-compatible SDK 連接 Port 18869：
```python
from openai import OpenAI
client = OpenAI(base_url="http://<NAS_IP>:18869/v1", api_key="any")
response = client.chat.completions.create(model="llm_new", messages=[...])
```

### API Key 工作模式說明

| 模式 | 待選池 | 輪換池 | 自動檢查 | 說明 |
|------|--------|--------|----------|------|
| **開啟 (Enabled)** | ✅ 自動加入 | ✅ 加入 | ❌ | 始終可用，直接進入輪換 |
| **關閉 (Disabled)** | ❌ 排除 | ❌ 排除 | ❌ | 完全停用 |
| **自動 (Auto)** | ✅ 通過測試後加入 | ✅ 檢查通過才加入 | ✅ 定期檢查，失敗則排除 | 自動管理健康狀態 |

## Docker 部署（NAS）

```bash
# 從原始碼構建
docker build -t llm-gateway:latest .

# 或使用 docker-compose
docker-compose up -d

# 自訂配置
# 編輯 config/config.yaml 後重啟容器
```

### Port 說明

| Port | 用途 |
|------|------|
| **18869** | API 代理端口，供客戶端調用 |
| **18866** | Web 管理介面 |

### 環境變數

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `DEBUG=true` | false | 啟用除錯日誌 |
| `TZ` | Asia/Taipei | 時區設定 |

### 配置檔案

`config/config.yaml` 儲存所有設定（YAML 格式）：

```yaml
servers:
  - id: "uuid"
    name: "OpenAI"
    api_url: "https://api.openai.com/v1"
    api_type: "openai"
    timeout: 0
server_models:
  - id: "uuid"
    server_id: "uuid"
    model_name: "GPT-4"
    model_id: "gpt-4"
server_api_keys:
  - id: "uuid"
    server_id: "uuid"
    api_key: "sk-..."       # 明文儲存（個人使用）
    status: "enabled"       # enabled / disabled / auto
    notes: "主要帳號"
local_model_maps:
  - id: "uuid"
    local_model: "llm_new"  # 客戶端看到的模型名稱
    server_model_id: "uuid" # 指向 server_models 的 id
settings:
  timeout: 5                # 全局超時（分鐘）
  connect_timeout: 30       # TCP 連接超時（秒）
  enable_negative_weight: true
  enable_retry: true
  max_retries: 3
  enable_retry_on_timeout: false
  enable_auto_check_api_key: true
  auto_check_interval_hours: 6
  weight_4xx: 5
  weight_5xx: 10
  timeout_weight: 8
  connect_timeout_weight: 12
  weight_reset_hours: 4
```

## 常見問題

### 為什麼 API Key 用明文儲存？

此專案設計為個人/NAS 使用，不涉及多使用者或多租戶場景。明文儲存簡化了配置管理，方便直接編輯 YAML 檔案。

### 如何測試 API Key 是否可用？

在「待選池 Pending Pool」頁面點擊 Key 旁的「測試 Test」按鈕，或使用「測試所選服務器所有 Key Test All」批量測試。結果以 LED 燈號顯示：
- ⚪ 灰色：未測試
- 🔵 藍色：通過
- 🔴 紅色：失敗

### 「待選池」是什麼？

自動管理的 Key 候選清單。`開啟 (Enabled)` 的 Key 自動加入，`自動 (Auto)` 的 Key 通過健康檢查後加入。可在待選池頁面對 Key 進行個別測試、查看最近測試記錄。

### LED 燈號說明

| 燈號 | 代表意義 |
|------|----------|
| ⚪ 灰色 | 尚未測試 |
| 🔵 藍色 | 測試通過 (ok/success) |
| 🔴 紅色 | 測試失敗 |

## 改版說明

### v2.5.1（2026-05-10）

- **測試模型可選**：待選池頁面加入模型下拉選單，連動所選服務器。支援單 Key 測試和批量測試使用指定模型。未選擇時自動使用服務器第一個模型（向後相容）
- **版本號更新**：v2.5.0 → v2.5.1

### v2.5.0（2026-05-10）

- **首頁顯示版本號**：系統狀態區塊加入版本號顯示，透過 `/api/version` API 取得
- **修復 Local Models 映射顯示為 UUID**：`loadModels()` 完成後才載入映射列表，避免競態條件導致服務器模型名稱顯示為 ID
- **修復 Health Check 使用錯誤模型**：`checkAllAPIKeys` 健康檢查改用服務器第一個模型，不再使用 `gpt-3.5-turbo` 硬編碼 fallback。單一測試、批量測試、背景檢查皆一致使用服務器支援的第一個模型
- **新增 `/api/version` API 端點**：回傳版本號和建置日期
- **版本號更新**：v2.4.0 → v2.5.0

### v2.4.0（2026-05-10）

- **頁面欄位順序調整**：
  - 模型頁（Server Models）：調整為「模型名稱 → 模型 ID → 關聯服務器 → 操作」
  - API Key 頁：調整為「API Key → 模式 → 狀態 → 關聯服務器 → 權重 → 待選池 → 備註 → 操作」
- **版本號更新**：v2.3.0 → v2.4.0

### v2.3.0（2026-05-10）

- **修復 chat completions 路徑 Bug**：修正 openai 類型 API URL 已含 `/v1` 時產生雙重 `/v1/v1` 路徑的問題（如 NVIDIA API）
- **修復 Debug 日誌格式錯誤**：`[UPSTREAM_REQUEST]` 日誌修正錯誤的 `%s/api/chat` 硬編碼
- **修復伺服器名稱競態條件**：模型頁面和 API Key 頁面因 AJAX 非同步載入順序不確定，導致伺服器名稱顯示為 UUID
- **Check Interval 限制**：設定頁面檢查間隔限制為 1~12 小時（原 3~168），預設 6 小時
- **連線池重用**：全域共享 HTTP Transport，所有 upstream client 共用連線池
- **模型/Key 新增驗證**：必須選擇服務器才能新增，前端和後端雙重檢查
- **待選池 Badge 修正**：`disabled` 模式顯示灰色「已停用」，不再誤顯示黃色「待測試」
- **待選池頁面優化**：進入頁面時自動載入最近測試記錄，測試完畢後同時刷新待選池和記錄
- **修復待選池列表消失 Bug**：`modeText` 和 `statusText` 函式移入全域模板，避免待選池頁面因函式未定義而無法渲染

### v1.2.0（2026-05-10）

- **UI 調整：模式/狀態分離**：原「狀態 Status」改為「模式 Mode」（開啟/關閉/自動），原「LED」改為「狀態 Status」（未測試灰/通過藍/失敗紅）
- **模式改為中文**：「enabled」→「開啟」,「disabled」→「關閉」,「auto」→「自動」，預設為「自動」
- **新增 statusText 函式**：在 API Key 列表和待選池列表中顯示「未測試」「通過 Pass」「失敗 Fail」文字
- **待選池列表欄位重新排列**：測試按鈕移至最左側（紫色），新增「測試結果」欄顯示錯誤訊息
- **批量測試不顯示結果區塊**：測試完直接更新列表，不再彈出結果表格
- **移除獨立測試結果區塊**：整合歷史記錄到待選池頁面下方

### v1.1.0（2026-05-10）

- **待選池與測試頁面整合**：測試結果直接顯示在待選池頁面下方（批量測試結果 + 最近測試記錄），移除獨立測試頁面
- **移除服務器獨立 Timeout**：新增服務器不再需要設定超時，統一使用全域 Timeout
- **測試功能集中到待選池**：所有 Key 測試（單一/批量）移至待選池頁面，API Key 頁面回歸純管理
- **待選池按服務器分組**：每個服務器獨立的表格卡片
- **待選池 Key 遮罩顯示**
- **修復單 Key 測試 model 選擇**：使用該 Key 所屬服務器的第一個模型

### v1.0.1（2026-05-09）

- **待選池自動化**：不再需要手動加入/移出待選池。根據 Key 的工作模式自動管理
- **API Key 編輯限制**：建立後只能修改備註和工作模式，服務器和 Key 值不可更改
- **修復單 Key 測試 405 錯誤**：改用 chat completions API（含隨機數學題）取代 GET /v1/models
- **加強 Debug 日誌**：加入 `[UPSTREAM_REQUEST]`、`[UPSTREAM_CALL_START]`、`[UPSTREAM_CALL_DONE]` 標記
- **加強說明文字**：Settings 頁面和服務器編輯表單補上 Timeout 功能說明
- **版本號更新**：v1.0.0 → v1.0.1

### v1.0.0（初始版本）

基於原始 LLM Gateway v0.12.8 重新編寫，修正已知缺陷：
- 引入 `sync` 和連線池管理，修復連線洩漏
- 統一模組導入，移除非使用套件
- 新增待選池（Pending Pool）功能
- 全新 Web UI，支援中英雙語和 LED 狀態顯示