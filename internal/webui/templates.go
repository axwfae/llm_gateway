package webui

import "html/template"

type PageData struct {
	Title   string
	Content template.HTML
}

const PageTemplate = `
{{define "main"}}
<!DOCTYPE html>
<html lang="zh-TW">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LLM Gateway - {{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; height: 100vh; background: #f5f5f5; }
        .sidebar { width: 200px; background: #2c3e50; color: white; padding: 20px; }
        .sidebar h1 { font-size: 20px; margin-bottom: 30px; padding-bottom: 10px; border-bottom: 1px solid #34495e; }
        .sidebar nav a { display: block; padding: 12px 15px; color: #bdc3c7; text-decoration: none; border-radius: 5px; margin-bottom: 5px; transition: all 0.2s; }
        .sidebar nav a:hover, .sidebar nav a.active { background: #34495e; color: white; }
        .main { flex: 1; padding: 30px; overflow-y: auto; }
        .card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .card h2 { font-size: 18px; margin-bottom: 15px; color: #2c3e50; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: 500; color: #555; }
        .form-group input, .form-group select { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 5px; font-size: 14px; }
        .form-group input:focus, .form-group select:focus { outline: none; border-color: #3498db; }
        button { padding: 10px 20px; background: #3498db; color: white; border: none; border-radius: 5px; cursor: pointer; font-size: 14px; }
        button:hover { background: #2980b9; }
        button.delete { background: #e74c3c; }
        button.delete:hover { background: #c0392b; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #eee; }
        th { background: #f8f9fa; font-weight: 600; color: #555; }
        .actions { display: flex; gap: 10px; }
        .modal { display: none; position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); }
        .modal.active { display: flex; align-items: center; justify-content: center; }
        .modal-content { background: white; padding: 30px; border-radius: 8px; width: 400px; max-width: 90%; }
        .modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .modal-header h3 { font-size: 18px; }
        .close { background: none; border: none; font-size: 24px; cursor: pointer; color: #999; }
        .btn-group { display: flex; gap: 10px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="sidebar">
        <h1>LLM Gateway</h1>
        <nav>
            <a href="/servers">服務器設置 Server Settings</a>
            <a href="/server-models">服務器模型設置 Server Models</a>
            <a href="/api-keys">API Key 設置 API Keys</a>
            <a href="/local-models">本地模型映射 Local Models</a>
            <a href="/settings">系統設置 Settings</a>
        </nav>
    </div>
    <div class="main">
        {{.Content}}
    </div>
    <script>
        function showModal(id) { document.getElementById(id).classList.add('active'); }
        function hideModal(id) { document.getElementById(id).classList.remove('active'); }
    </script>
</body>
</html>
{{end}}
`

const ServersPage = `
<div class="card">
    <h2>服務器設置 Server Settings</h2>
    <button onclick="showModal('addServerModal')">新增服務器 Add Server</button>
</div>
<div class="card">
    <table>
        <thead>
            <tr>
                <th>名稱 Name</th>
                <th>API URL</th>
                <th>API 類型 API Type</th>
                <th>操作 Actions</th>
            </tr>
        </thead>
        <tbody id="serversTable"></tbody>
    </table>
</div>

<div class="modal" id="addServerModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>新增服務器 Add Server</h3>
            <button class="close" onclick="hideModal('addServerModal')">&times;</button>
        </div>
        <div class="form-group">
            <label>服務器名稱 Server Name</label>
            <input type="text" id="serverName" placeholder="例如: OpenAI / e.g. OpenAI">
        </div>
        <div class="form-group">
            <label>API URL</label>
            <input type="text" id="serverAPIURL" placeholder="例如: https://api.openai.com/v1 / e.g. https://api.openai.com/v1">
        </div>
        <div class="form-group">
            <label>API 類型 API Type</label>
            <select id="serverAPIType">
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="deepseek">DeepSeek</option>
                <option value="ollama">Ollama</option>
                <option value="other">Other</option>
            </select>
        </div>
        <div class="btn-group">
            <button onclick="addServer()">新增 Add</button>
            <button onclick="hideModal('addServerModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>
<script>
function loadServers() {
    fetch('/api/servers').then(r=>r.json()).then(d=>{
        const tbody = document.getElementById('serversTable');
        tbody.innerHTML = d.servers.map(s=>'<tr><td>'+s.name+'</td><td>'+s.api_url+'</td><td>'+s.api_type+'</td><td class="actions"><button class="delete" onclick="deleteServer(\''+s.id+'\')">刪除 Delete</button></td></tr>').join('');
    });
}
function addServer() {
    fetch('/api/servers', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        name: document.getElementById('serverName').value,
        api_url: document.getElementById('serverAPIURL').value,
        api_type: document.getElementById('serverAPIType').value
    })}).then(r=>{if(r.ok){hideModal('addServerModal');loadServers();}else{r.json().then(d=>alert(d.error||'新增失敗 Add Failed'));}});
}
function deleteServer(id) {
    if(confirm('確定要刪除? / Confirm delete?')) fetch('/api/servers/'+id,{method:'DELETE'}).then(loadServers);
}
loadServers();
</script>
`

const ServerModelsPage = `
<div class="card">
    <h2>服務器模型設置 Server Models</h2>
    <button onclick="showModal('addModelModal')">新增模型 Add Model</button>
</div>
<div class="card">
    <table>
        <thead>
            <tr>
                <th>模型名稱 Model Name</th>
                <th>關聯服務器 Server</th>
                <th>模型 ID Model ID</th>
                <th>操作 Actions</th>
            </tr>
        </thead>
        <tbody id="modelsTable"></tbody>
    </table>
</div>

<div class="modal" id="addModelModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>新增模型 Add Model</h3>
            <button class="close" onclick="hideModal('addModelModal')">&times;</button>
        </div>
        <div class="form-group">
            <label>選擇服務器 Select Server</label>
            <select id="modelServerID">
                <option value="">請選擇服務器 Select Server</option>
            </select>
        </div>
        <div class="form-group">
            <label>模型名稱 Model Name (Display)</label>
            <input type="text" id="modelName" placeholder="例如: GPT-4 / e.g. GPT-4">
        </div>
        <div class="form-group">
            <label>模型 ID Model ID (API)</label>
            <input type="text" id="modelID" placeholder="例如: gpt-4 / e.g. gpt-4">
        </div>
        <div class="btn-group">
            <button onclick="addServerModel()">新增 Add</button>
            <button onclick="hideModal('addModelModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>
<script>
function loadServers() {
    fetch('/api/servers').then(r=>r.json()).then(d=>{
        const sel = document.getElementById('modelServerID');
        sel.innerHTML = '<option value="">請選擇服務器 Select Server</option>'+d.servers.map(s=>'<option value="'+s.id+'">'+s.name+'</option>').join('');
    });
}
function loadModels() {
    fetch('/api/server-models').then(r=>r.json()).then(d=>{
        const tbody = document.getElementById('modelsTable');
        fetch('/api/servers').then(r=>r.json()).then(s=>{
            const servers = {}; s.servers.forEach(x=>servers[x.id]=x.name);
            tbody.innerHTML = d.server_models.map(m=>'<tr><td>'+m.model_name+'</td><td>'+(servers[m.server_id]||'')+'</td><td>'+m.model_id+'</td><td class="actions"><button class="delete" onclick="deleteServerModel(\''+m.id+'\')">刪除 Delete</button></td></tr>').join('');
        });
    });
}
function addServerModel() {
    fetch('/api/server-models', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        server_id: document.getElementById('modelServerID').value,
        model_name: document.getElementById('modelName').value,
        model_id: document.getElementById('modelID').value
    })}).then(r=>{if(r.ok){hideModal('addModelModal');loadModels();}else{r.json().then(d=>alert(d.error||'新增失敗 Add Failed'));}});
}
function deleteServerModel(id) {
    if(confirm('確定要刪除? / Confirm delete?')) fetch('/api/server-models/'+id,{method:'DELETE'}).then(loadModels);
}
loadServers();loadModels();
</script>
`

const APIKeysPage = `
<div class="card">
    <h2>API Key 設置 API Keys</h2>
    <button onclick="showModal('addKeyModal')">新增 API Key</button>
</div>
<div class="card">
    <table>
        <thead>
            <tr>
                <th>關聯服務器 Server</th>
                <th>API Key</th>
                <th>操作 Actions</th>
            </tr>
        </thead>
        <tbody id="keysTable"></tbody>
    </table>
</div>

<div class="modal" id="addKeyModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>新增 API Key</h3>
            <button class="close" onclick="hideModal('addKeyModal')">&times;</button>
        </div>
        <div class="form-group">
            <label>選擇服務器 Select Server</label>
            <select id="keyServerID">
                <option value="">請選擇服務器 Select Server</option>
            </select>
        </div>
        <div class="form-group">
            <label>API Key</label>
            <input type="text" id="keyValue" placeholder="請輸入 API Key / Enter API Key">
        </div>
        <div class="btn-group">
            <button onclick="addAPIKey()">新增 Add</button>
            <button onclick="hideModal('addKeyModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>
<script>
function loadServers() {
    fetch('/api/servers').then(r=>r.json()).then(d=>{
        const sel = document.getElementById('keyServerID');
        sel.innerHTML = '<option value="">請選擇服務器 Select Server</option>'+d.servers.map(s=>'<option value="'+s.id+'">'+s.name+'</option>').join('');
    });
}
function loadKeys() {
    fetch('/api/server-api-keys').then(r=>r.json()).then(d=>{
        const tbody = document.getElementById('keysTable');
        fetch('/api/servers').then(r=>r.json()).then(s=>{
            const servers = {}; s.servers.forEach(x=>servers[x.id]=x.name);
            tbody.innerHTML = d.server_api_keys.map(k=>'<tr><td>'+(servers[k.server_id]||'')+'</td><td>'+k.api_key+'</td><td class="actions"><button class="delete" onclick="deleteAPIKey(\''+k.id+'\')">刪除 Delete</button></td></tr>').join('');
        });
    });
}
function addAPIKey() {
    fetch('/api/server-api-keys', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        server_id: document.getElementById('keyServerID').value,
        api_key: document.getElementById('keyValue').value,
        is_active: true
    })}).then(r=>{if(r.ok){hideModal('addKeyModal');loadKeys();}else{r.json().then(d=>alert(d.error||'新增失敗 Add Failed'));}});
}
function deleteAPIKey(id) {
    if(confirm('確定要刪除? / Confirm delete?')) fetch('/api/server-api-keys/'+id,{method:'DELETE'}).then(loadKeys);
}
loadServers();loadKeys();
</script>
`

const LocalModelsPage = `
<div class="card">
    <h2>本地模型映射 Local Models</h2>
    <button onclick="showModal('addMapModal')">新增映射 Add Mapping</button>
</div>
<div class="card">
    <table>
        <thead>
            <tr>
                <th>本地模型名稱 Local Model</th>
                <th>映射到服務器模型 Server Model</th>
                <th>操作 Actions</th>
            </tr>
        </thead>
        <tbody id="mapsTable"></tbody>
    </table>
</div>

<div class="modal" id="addMapModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>新增映射 Add Mapping</h3>
            <button class="close" onclick="hideModal('addMapModal')">&times;</button>
        </div>
        <div class="form-group">
            <label>選擇服務器模型 Select Server Model</label>
            <select id="serverModelID">
                <option value="">請選擇模型 Select Model</option>
            </select>
        </div>
        <div class="form-group">
            <label>本地模型名稱 Local Model Name</label>
            <input type="text" id="localModel" placeholder="例如: gpt-4 / e.g. gpt-4">
        </div>
        <div class="btn-group">
            <button onclick="addMapping()">新增 Add</button>
            <button onclick="hideModal('addMapModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>
<script>
function loadServerModels() {
    fetch('/api/server-models').then(r=>r.json()).then(d=>{
        const sel = document.getElementById('serverModelID');
        sel.innerHTML = '<option value="">請選擇模型 Select Model</option>'+d.server_models.map(m=>'<option value="'+m.id+'">'+m.model_name+' ('+m.model_id+')</option>').join('');
    });
}
function loadMaps() {
    fetch('/api/local-model-maps').then(r=>r.json()).then(d=>{
        const tbody = document.getElementById('mapsTable');
        fetch('/api/server-models').then(r=>r.json()).then(m=>{
            const models = {}; m.server_models.forEach(x=>models[x.id]=x.model_name);
            tbody.innerHTML = d.local_model_maps.map(x=>'<tr><td>'+x.local_model+'</td><td>'+(models[x.server_model_id]||'')+'</td><td class="actions"><button class="delete" onclick="deleteMapping(\''+x.id+'\')">刪除 Delete</button></td></tr>').join('');
        });
    });
}
function addMapping() {
    fetch('/api/local-model-maps', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        local_model: document.getElementById('localModel').value,
        server_model_id: document.getElementById('serverModelID').value
    })}).then(r=>{if(r.ok){hideModal('addMapModal');loadMaps();}else{r.json().then(d=>alert(d.error||'新增失敗 Failed'));}});
}
function deleteMapping(id) {
    if(confirm('確定要刪除? / Confirm delete?')) fetch('/api/local-model-maps/'+id,{method:'DELETE'}).then(loadMaps);
}
loadServerModels();loadMaps();
</script>
`

const SettingsPage = `
<div class="card">
    <h2>系統設置 Settings</h2>
    <div class="form-group">
        <label>版本號 Version</label>
        <input type="text" id="appVersion" value="" disabled>
    </div>
    <div class="form-group">
        <label>API 端口 API Port</label>
        <input type="text" id="apiPort" value="" disabled>
    </div>
    <div class="form-group">
        <label>Web UI 端口 Web Port</label>
        <input type="text" id="uiPort" value="" disabled>
    </div>
    <div class="form-group">
        <label>Timeout (分鐘 Minutes) (3-15)</label>
        <input type="number" id="timeout" min="3" max="15" value="5">
    </div>
    <div style="margin-top:20px;">
        <button onclick="saveSettings()">儲存設定 Save Settings</button>
        <button onclick="restartSystem()">重啟 llm_gateway Restart</button>
    </div>
    <p style="color:#7f8c8d;margin-top:10px;">說明：當有多個 API Key 時，每次請求會輪流使用不同的 Key，以平均分配使用量。<br>Note: When multiple API Keys exist, each request uses a different key in rotation to balance usage.</p>
</div>
<script>
fetch('/api/settings').then(r=>r.json()).then(d=>{
    if(d.settings) {
        document.getElementById('timeout').value = d.settings.timeout || 5;
    }
    if(d.version) {
        document.getElementById('appVersion').value = d.version;
    }
    if(d.ports) {
        document.getElementById('apiPort').value = d.ports.api;
        document.getElementById('uiPort').value = d.ports.ui;
    }
});
function saveSettings() {
    var timeout = parseInt(document.getElementById('timeout').value);
    if(timeout < 3) timeout = 3;
    if(timeout > 15) timeout = 15;
    fetch('/api/settings', {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({timeout: timeout})
    }).then(r=>r.json()).then(d=>{
        alert(d.status === 'ok' ? '設定已儲存 Settings Saved' : '儲存失敗 Failed');
    });
}
function restartSystem() {
    if(confirm('確定要重啟 llm_gateway? / Confirm restart?')) {
        fetch('/api/reload', {method:'POST'}).then(r=>r.json()).then(d=>{
            alert(d.message || '完成 Done');
        });
    }
}
</script>
`

const IndexPage = `
<div class="card">
    <h2>歡迎使用 LLM Gateway Welcome</h2>
    <p>請從左側選單選擇功能進行配置。Please select a function from the left menu to configure.</p>
    <ul style="margin-top:20px;padding-left:20px;">
        <li>服務器設置 - 管理 LLM 伺服器 Server Settings - Manage LLM Servers</li>
        <li>服務器模型設置 - 配置各伺服器的模型 Server Models - Configure server models</li>
        <li>API Key 設置 - 管理 API Keys (支援多 Key 輪換) API Keys - Manage (supports key rotation)</li>
        <li>本地模型映射 - 將本地模型名稱映射到伺服器模型 Local Models - Map local model names to server models</li>
    </ul>
</div>
`

var Templates = template.Must(template.New("webui").Parse(PageTemplate + ServersPage + ServerModelsPage + APIKeysPage + LocalModelsPage + SettingsPage + IndexPage))