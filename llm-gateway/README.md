# LLM Gateway v2.6.6

輕量級 LLM API 代理閘道服務，支援多後端路由、API Key 輪換、錯誤重試、SSE Streaming 與 Web 管理介面。

## 新特性 (v2.6.0)

- **SSE Streaming 支援** — 完整支援串流回應 (`stream: true`)，實現即時對話
- **效能優化** — API Key 權重與檢查結果改為記憶體內操作，不再每次寫入 YAML，大幅降低高併發下的 I/O 開銷
- **模組化重構** — 單一 `main.go` (1133 行) 拆分為專用套件，提升可維護性
- **模板分離** — HTML/CSS/JS 移到獨立檔案並使用 Go `embed` 打包
- **錯誤修正** — 修復重試循環中 `defer resp.Body.Close()` 導致文件描述符洩漏的問題

## 工作原理

```
客戶端 (OpenAI-compatible SDK)
       │
       │ POST /v1/chat/completions {model: "llm_new", stream: true/false}
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
│  7. 串流模式: SSE 即時轉發       │
│  8. 失敗時自動重試/換 Key       │
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
│  ├ 映射 Mapping                 │
│  └ 系統設定 Settings            │
└─────────────────────────────────┘
```

### 核心功能

| 功能 | 說明 |
|------|------|
| **多後端路由** | 透過「本地模型名稱 → 服務器模型」映射，一個端點路由到不同 LLM 後端 |
| **API Key 輪換** | 兩種模式：**Round-Robin** 依序輪換；**負權重** 選權重最低的 Key，選定後黏著使用直到報錯才換，權重相同時選最近最少使用的 Key（依最後使用時間 → 亂數） |
| **錯誤重試** | 網路錯誤/超時/4xx/5xx 自動重試（可設定重試次數），每次更換 Key |
| **SSE Streaming** | 完整支援串流回應，即時轉發上游 SSE 資料到客戶端 |
| **分離式超時** | TCP 連接超時和響應超時分開設定，避免慢連接佔用過久 |
| **自動健康檢查** | 定期測試 `auto` 模式的 Key，失敗的 Key 自動排除出輪換池 |
| **待選池** | 根據 Key 的工作模式自動管理。`開啟` 自動加入、`關閉` 排除、`自動` 通過測試後加入 |
| **中英雙語 UI** | 所有頁面同時顯示中文和英文標籤 |
| **連線池共享** | 代理請求 (`/v1/chat/completions` 含 streaming) 共用連線池，減少 TCP 連線建立開銷。健康檢查測試和通用代理 (`/v1/`) 使用獨立連線 |
 
### API 端點

**代理端點** (Port 18869)：

| 路徑 | 功能 |
|------|------|
| `/v1/chat/completions` | OpenAI-compatible 聊天補全（支援 streaming） |
| `/v1/models` | 列出可用模型 |
| `/v1/` | 通用代理（直接轉發） |
| `/health` | 健康檢查 |

**管理端點** (Port 18866)：

| 方法 | 路徑 | 功能 |
|------|------|------|
| GET/POST/PUT/DELETE | `/api/servers` | 服務器 CRUD |
| GET/POST/PUT/DELETE | `/api/server-models` | 模型 CRUD |
| GET/POST/PUT/DELETE | `/api/server-api-keys` | API Key CRUD |
| GET/POST/PUT/DELETE | `/api/local-model-maps` | 映射 CRUD |
| GET | `/api/pending-pool` | 列出待選池 Key |
| GET/POST | `/api/settings` | 系統設定讀寫 |
| POST | `/api/reload` | 重新加載配置文件 |
| POST | `/api/reset-weights` | 重置所有 Key 權重 |
| POST/GET | `/api/test-key` | 單一/批量測試 API Key |
| GET | `/api/test-results` | 取得測試結果 |
| GET | `/api/version` | 取得版本號 |

### Port 說明

| Port | 用途 |
|------|------|
| **18869** | API 代理端口，供客戶端調用 |
| **18866** | Web 管理介面 + Management REST API |

## 專案結構

```
llm-gateway/
├── cmd/
│   └── main.go                    # 輕量入口，路由註冊
├── internal/
│   ├── api/
│   │   ├── handlers.go            # Web UI CRUD handlers + Settings
│   │   └── test.go                # API Key 測試端點
│   ├── proxy/
│   │   ├── proxy.go               # 代理核心 (chat, stream, models)
│   │   └── error.go               # 錯誤分類與權重計算
│   ├── storage/
│   │   ├── storage.go             # 資料層 (config + 記憶體運行時狀態)
│   │   └── tester.go              # API Key 測試邏輯（給 auto 即時測試用）
│   ├── webui/
│   │   ├── templates.go           # 模板載入器 (Go embed)
│   │   └── pages/                 # 獨立 HTML 模板檔案
│   │       ├── layout.html        # 主版型
│   │       ├── index.html         # 首頁
│   │       ├── servers.html       # 服務器管理
│   │       ├── models.html        # 模型管理
│   │       ├── apikeys.html       # API Key 管理（含測試功能）
│   │       ├── localmodels.html   # 映射管理
│   │       ├── settings.html      # 系統設定
│   │       └── testresults.html   # 測試結果
│   ├── utils/
│   │   └── utils.go               # 工具函數
│   └── version/
│       └── version.go             # 版本資訊
├── config/
│   └── config.yaml                # 設定檔
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

## 快速開始

### 1. 設定服務器

1. 開啟 Web UI `http://<NAS_IP>:18866`
2. 進入「服務器 Servers」頁面，新增 LLM 後端
3. 支援類型：`openai`、`anthropic`、`ollama`

### 2. 設定模型

2. 進入「模型 Models」頁面，為每個服務器新增支援的模型

### 3. 加入 API Key

3. 進入「API Keys」頁面，新增 API Key
   - **開啟 (Enabled)**: 直接加入待選池和輪換池
   - **關閉 (Disabled)**: 完全排除
   - **自動 (Auto)**: 首次請求時自動測試，通過才加入輪換（失敗則排除，後續由定期健康檢查更新狀態）

### 4. 設定映射

4. 進入「映射 Mapping」頁面，設定本地模型名稱到服務器模型的映射

### 5. 開始使用

透過 curl：
```bash
curl http://localhost:18869/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "my-model", "messages": [{"role": "user", "content": "Hello"}]}'

# Streaming 模式
curl http://localhost:18869/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "my-model", "stream": true, "messages": [{"role": "user", "content": "Hello"}]}'
```

或使用 OpenAI Python SDK：
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

### API Key 選擇演算法

支援兩種模式，透過系統設定中的「啟用負權重模式」切換。Key 的可用性根據模式判定：

| 模式 | 測試結果 | 加入待選？ | 說明 |
|------|---------|-----------|------|
| 關閉 | — | ❌ | 完全停用 |
| 開啟 | — | ✅ | 始終可用 |
| 自動 | 未測試 | 觸發即時測試 | 首次請求時自動送出測試，通過才加入 |
| 自動 | 通過 | ✅ | 健康檢查正常 |
| 自動 | 失敗 | ❌ | 自動退出輪換 |

> 模式為「自動」且尚未測試的 Key，在第一次被搜尋時會自動發送測試請求，根據結果決定是否加入待選池。測試通過前該請求會等待約 1-2 秒。

**Round-Robin 模式**（關閉負權重）

同一服務器下的所有啟用 Key 依序輪換，每次請求換一把。適合所有 Key 額度、延遲相近的情境。

**負權重模式**（啟用負權重）

選擇流程如下：

```
請求到達 → 該服務器有 currentKey 且仍有效？
  ├─ 是 → 直接使用（黏著），不重新挑選
  └─ 否 → 從待選池中篩選：
           1. 挑選 NegativeWeight 最低的 Key
           2. 若多把權重相同 → 挑 lastUsedTime 最久遠的（最近最少使用）
           3. 若仍相同 → 亂數選一把
           4. 設為 currentKey，記錄 lastUsedTime
```

黏著機制：選定後持續使用同一把 Key，直到請求報錯才會重新挑選。報錯時該 Key 的權重會增加（4xx/5xx/timeout 各有不同權重），下一次請求就會選到別把。

| 事件 | 行為 |
|------|------|
| 請求成功 | currentKey 不變，持續使用 |
| 請求失敗（4xx/5xx/逾時） | 加權重 → 清除 currentKey → retry 換 Key |
| 所有 retry 失敗 | 清除 currentKey → 下一次請求重新挑選 |
| 管理員手動重設權重 | 所有 Key 權重歸零 → 重新公平競爭 |

### 環境變數

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `DEBUG=true` | false | 啟用除錯日誌 |
| `TZ` | Asia/Taipei | 時區設定 |

## Docker 部署

```bash
docker-compose up -d
```

或手動構建：

```bash
docker build -t llm-gateway:v2.6.6 .
docker run -d \
  -p 18869:18869 \
  -p 18866:18866 \
  -v $(pwd)/config:/app/config \
  llm-gateway:v2.6.6
```

## 設定檔

`config/config.yaml` 儲存所有設定（YAML 格式）：

```yaml
servers:
  - id: "svr_xxx"
    name: "My OpenAI"
    api_url: "https://api.openai.com/v1"
    api_type: "openai"
    timeout: 0                    # 分鐘，0=使用全域設定
server_models:
  - id: "mdl_xxx"
    server_id: "svr_xxx"
    model_name: "GPT-4"           # 顯示名稱
    model_id: "gpt-4"             # 實際上游模型 ID
server_api_keys:
  - id: "key_xxx"
    server_id: "svr_xxx"
    api_key: "sk-..."             # 明文儲存（個人使用）
    status: "enabled"             # enabled / disabled / auto
    notes: "主要帳號"
local_model_maps:
  - id: "map_xxx"
    local_model: "my-model"       # 客戶端看到的模型名稱
    server_model_id: "mdl_xxx"    # 指向 server_models 的 id
settings:
  timeout: 5                      # 全域超時（分鐘）
  connect_timeout: 30             # TCP 連接超時（秒）
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

> 注意：API Key 的 `negative_weight`、`last_check_time`、`last_check_result` 等運行時狀態不再持久化到 YAML，改為記憶體內管理，重啟後會重新計算。
>
> 在 Web UI 修改 `settings` 後會即時生效，無需重啟服務。若直接編輯 YAML 檔案，需透過 Web UI 的「重新加載配置」按鈕或 `POST /api/reload` 生效。

## 常見問題

### 為什麼 API Key 用明文儲存？

此專案設計為個人/NAS 使用，不涉及多使用者或多租戶場景。明文儲存簡化了配置管理，方便直接編輯 YAML 檔案。

### 如何測試 API Key 是否可用？

在「API Keys」頁面點擊 Key 旁的「測試 Test」按鈕，或使用「批量測試 Batch Test」功能。結果以 LED 燈號顯示：

| 燈號 | 代表意義 |
|------|----------|
| ⚪ 灰色 | 尚未測試 |
| 🔵 藍色 | 測試通過 (ok/success) |
| 🔴 紅色 | 測試失敗 |

### 「待選池」是什麼？

自動管理的 Key 候選清單。`開啟 (Enabled)` 的 Key 自動加入，`自動 (Auto)` 的 Key 通過測試後加入。可在「API Keys」頁面對 Key 進行個別測試或批量測試。

### 請求處理流程

```
/v1/chat/completions POST {model: "llm_new", stream: true}
  → 提取 model 字段
  → 查找 LocalModelMap("llm_new") → 得到 ServerModelID
  → 查找 ServerModel(id) → 得到 ServerID + 實際 ModelID
  → 查找 Server(id) → 得到 API URL + API Type
  → GetNextAPIKey(serverID):
  │  Round-Robin 模式：依序輪換 Key
  │  負權重模式：黏著使用目前 Key（若無則挑權重最低的），
  │             權重相同時選最久未使用的，再相同則亂數
  → 替換 model 為 ModelID
  → 根據 API Type 決定 upstream 路徑：
      openai   → {api_url}/chat/completions
                 (若 api_url 不含 /v1 則自動補上)
      anthropic → {api_url}/v1/messages
      ollama   → {api_url}/api/chat
  → 發送請求到上游
  → 串流模式：SSE 即時轉發（chunked transfer），支援中途取消
  → 非串流模式：完整回應後回傳
  → 失敗 → 加權重 → 換 Key → 重試（最多 MaxRetries 次）
```

## 改版說明

### v2.6.6（2026-05-14）

- **修復定時健康檢查不會重試已失敗的 Key**：`checkAllAPIKeys` 改用 `GetServerAPIKeys()` 取代 `GetServerAPIKeysByServer()`，確保已失敗的 auto Key 仍會被定期重新測試，通過後自動恢復輪換
- **版號更新**：v2.6.5 → v2.6.6

### v2.6.5（2026-05-13）

- **自動測試未測試的 auto Key**：搜尋可用 Key 時若發現模式為「自動」且從未測試，立即發送測試請求判定是否加入待選池
- **統一測試日誌格式**：單一測試與批量測試共用相同 `[DEBUG] test-key` 格式，均包含 model 資訊
- **重構測試邏輯**：將測試函數移至 `internal/storage/tester.go`，消除套件間依賴
- **版號更新**：v2.6.4 → v2.6.5

### v2.6.4（2026-05-13）

- **首頁文字修正**：移除已刪除的待選池頁面連結
- **DEBUG 日誌補全**：單一 Key 測試和批次測試每把 Key 均輸出 debug log
- **修復名稱顯示 Race Condition**：apikeys/localmodels 頁面確保 lookup map 填充後才渲染列表，避免名稱顯示為 ID
- **版號更新**：v2.6.3 → v2.6.4

### v2.6.3（2026-05-13）

- **狀態/結果欄位合併**：api-keys 列表「狀態」與「結果」合併，依模式+測試結果顯示 LED（關閉灰/開啟藍/自動通過藍/自動失敗紅）
- **Key 選擇邏輯更新**：auto+測試失敗的 Key 不再加入待選池；後端自動從 Status 推導 IsActive
- **Settings 頁面重排**：欄位依關聯性重新分組，每項設定附加說明文字
- **移除待選池頁面**：功能已整合至 api-keys 頁面
- **版號更新**：v2.6.2 → v2.6.3

### v2.6.2（2026-05-13）

- **UI 欄位排序**：server-models 列表改為「服務器→模型ID→名稱」，新增表單改為「服務器→模型ID→名稱」
- **IsActive 自動推導**：api-keys 編輯表單移除「啟用」checkbox，後端從 Status 自動推導
- **移除待選池頁面**：路由、模板、側邊欄連結一併移除
- **修復 local-models 載入**：分離模型與映射載入，避免互相阻塞
- **版號更新**：v2.6.1 → v2.6.2

### v2.6.1（2026-05-13）

- **UI 修正**：server-models/pending-pool 服務器欄位改顯示名稱而非 ID；local-models 模型欄位改顯示名稱而非 ID
- **新增 API Key 簡化**：新增表單移除狀態選項（僅保留模式），狀態與測試結果整合顯示
- **批量測試優化**：選擇服務器後自動加載該服務器的模型下拉清單
- **版號更新**：v2.6.0 → v2.6.1

### v2.6.0（2026-05-13）

- **SSE Streaming 支援** — 完整支援串流回應 (`stream: true`)，即時轉發上游資料
- **效能優化** — API Key 權重與檢查結果改為記憶體內操作，不再每次寫入 YAML
- **模組化重構** — `cmd/main.go` 拆分為 `internal/api/`、`internal/proxy/`、`internal/webui/pages/` 等專用套件
- **模板分離** — HTML/CSS/JS 從 Go 常量字串移到獨立檔案，使用 `embed` 打包
- **錯誤修正** — 修復重試循環中 `defer resp.Body.Close()` 導致文件描述符洩漏
- **版本號更新**：v2.5.1 → v2.6.0

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
- **測試功能集中到待選池**：所有 Key 測試（單一/批量）移至待選池頁面，API Key 頁面僅保留配置功能
- **頁面頂部版本標籤**：v1.0.0 → v1.1.0
