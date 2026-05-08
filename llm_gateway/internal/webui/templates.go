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
                <th>超時 Timeout (分)</th>
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
        <div class="form-group">
            <label>超時 Timeout (分鐘 Minutes) (0=使用全局, 3-15)</label>
            <input type="number" id="serverTimeout" min="0" max="15" value="0" placeholder="0 = 使用全局設置">
        </div>
        <div class="btn-group">
            <button onclick="addServer()">新增 Add</button>
            <button onclick="hideModal('addServerModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>

<div class="modal" id="editServerModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>編輯服務器 Edit Server</h3>
            <button class="close" onclick="hideModal('editServerModal')">&times;</button>
        </div>
        <input type="hidden" id="editServerId">
        <div class="form-group">
            <label>服務器名稱 Server Name</label>
            <input type="text" id="editServerName">
        </div>
        <div class="form-group">
            <label>API URL</label>
            <input type="text" id="editServerAPIURL">
        </div>
        <div class="form-group">
            <label>API 類型 API Type</label>
            <select id="editServerAPIType">
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="deepseek">DeepSeek</option>
                <option value="ollama">Ollama</option>
                <option value="other">Other</option>
            </select>
        </div>
        <div class="form-group">
            <label>超時 Timeout (分鐘 Minutes) (0=使用全局, 3-15)</label>
            <input type="number" id="editServerTimeout" min="0" max="15" value="0">
        </div>
        <div class="btn-group">
            <button onclick="updateServer()">儲存 Save</button>
            <button onclick="hideModal('editServerModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>
<script>
function loadServers() {
    fetch('/api/servers').then(r=>r.json()).then(d=>{
        const tbody = document.getElementById('serversTable');
        tbody.innerHTML = d.servers.map(s=>'<tr><td>'+s.name+'</td><td>'+s.api_url+'</td><td>'+s.api_type+'</td><td>'+(s.timeout||0)+'</td><td class="actions"><button onclick="editServer(\''+s.id+'\',\''+s.name+'\',\''+s.api_url+'\',\''+s.api_type+'\','+s.timeout+')">編輯 Edit</button><button class="delete" onclick="deleteServer(\''+s.id+'\')">刪除 Delete</button></td></tr>').join('');
    });
}
function addServer() {
    var timeout = parseInt(document.getElementById('serverTimeout').value);
    if(timeout < 0) timeout = 0;
    if(timeout > 15) timeout = 15;
    fetch('/api/servers', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        name: document.getElementById('serverName').value,
        api_url: document.getElementById('serverAPIURL').value,
        api_type: document.getElementById('serverAPIType').value,
        timeout: timeout
    })}).then(r=>{if(r.ok){hideModal('addServerModal');loadServers();}else{r.json().then(d=>alert(d.error||'新增失敗 Add Failed'));}});
}
function editServer(id, name, api_url, api_type, timeout) {
    document.getElementById('editServerId').value = id;
    document.getElementById('editServerName').value = name;
    document.getElementById('editServerAPIURL').value = api_url;
    document.getElementById('editServerAPIType').value = api_type;
    document.getElementById('editServerTimeout').value = timeout || 0;
    showModal('editServerModal');
}
function updateServer() {
    var timeout = parseInt(document.getElementById('editServerTimeout').value);
    if(timeout < 0) timeout = 0;
    if(timeout > 15) timeout = 15;
    fetch('/api/servers', {method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        id: document.getElementById('editServerId').value,
        name: document.getElementById('editServerName').value,
        api_url: document.getElementById('editServerAPIURL').value,
        api_type: document.getElementById('editServerAPIType').value,
        timeout: timeout
})}).then(r=>{if(r.ok){hideModal('editServerModal');loadServers();}else{r.json().then(d=>alert(d.error||'儲存失敗 Save Failed'));}});
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

<div class="modal" id="editModelModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>編輯模型 Edit Model</h3>
            <button class="close" onclick="hideModal('editModelModal')">&times;</button>
        </div>
        <input type="hidden" id="editModelId">
        <div class="form-group">
            <label>選擇服務器 Select Server</label>
            <select id="editModelServerID">
                <option value="">請選擇服務器 Select Server</option>
            </select>
        </div>
        <div class="form-group">
            <label>模型名稱 Model Name (Display)</label>
            <input type="text" id="editModelName">
        </div>
        <div class="form-group">
            <label>模型 ID Model ID (API)</label>
            <input type="text" id="editModelID">
        </div>
        <div class="btn-group">
            <button onclick="updateServerModel()">儲存 Save</button>
            <button onclick="hideModal('editModelModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>
<script>
var availableServers = [];
function loadServers() {
    fetch('/api/servers').then(r=>r.json()).then(d=>{
        availableServers = d.servers;
        const sel = document.getElementById('modelServerID');
        sel.innerHTML = '<option value="">請選擇服務器 Select Server</option>'+d.servers.map(s=>'<option value="'+s.id+'">'+s.name+'</option>').join('');
        const editSel = document.getElementById('editModelServerID');
        editSel.innerHTML = '<option value="">請選擇服務器 Select Server</option>'+d.servers.map(s=>'<option value="'+s.id+'">'+s.name+'</option>').join('');
    });
}
function loadModels() {
    fetch('/api/server-models').then(r=>r.json()).then(d=>{
        const tbody = document.getElementById('modelsTable');
        fetch('/api/servers').then(r=>r.json()).then(s=>{
            const servers = {}; s.servers.forEach(x=>servers[x.id]=x.name);
            tbody.innerHTML = d.server_models.map(m=>'<tr><td>'+m.model_name+'</td><td>'+(servers[m.server_id]||'')+'</td><td>'+m.model_id+'</td><td class="actions"><button onclick="editServerModel(\''+m.id+'\',\''+m.server_id+'\',\''+m.model_name+'\',\''+m.model_id+'\')">編輯 Edit</button><button class="delete" onclick="deleteServerModel(\''+m.id+'\')">刪除 Delete</button></td></tr>').join('');
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
function editServerModel(id, serverId, modelName, modelId) {
    document.getElementById('editModelId').value = id;
    document.getElementById('editModelServerID').value = serverId;
    document.getElementById('editModelName').value = modelName;
    document.getElementById('editModelID').value = modelId;
    showModal('editModelModal');
}
function updateServerModel() {
    fetch('/api/server-models', {method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        id: document.getElementById('editModelId').value,
        server_id: document.getElementById('editModelServerID').value,
        model_name: document.getElementById('editModelName').value,
        model_id: document.getElementById('editModelID').value
    })}).then(r=>{if(r.ok){hideModal('editModelModal');loadModels();}else{r.json().then(d=>alert(d.error||'儲存失敗 Save Failed'));}});
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
    <button onclick="testAllAPIKeys()">測試所有 API Key Test All API Keys</button>
</div>
<div class="card">
    <table>
        <thead>
            <tr>
                <th>關聯服務器 Server</th>
                <th>狀態 Status</th>
                <th>API Key</th>
                <th>Status LED</th>
                <th>Notes</th>
                <th>負權重 Weight</th>
                <th>剩餘重置時間 (分鐘) / Remaining Reset Time (min)</th>
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
        <div class="form-group">
            <label>Status</label>
            <select id="keyStatus">
                <option value="enabled">Enabled</option>
                <option value="disabled">Disabled</option>
                <option value="auto">Auto</option>
            </select>
        </div>
        <div class="form-group">
            <label>Notes</label>
            <input type="text" id="keyNotes" placeholder="備註 / Notes">
        </div>
        <div class="btn-group">
            <button onclick="addAPIKey()">新增 Add</button>
            <button onclick="hideModal('addKeyModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>

<div class="modal" id="editKeyModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>編輯 API Key</h3>
            <button class="close" onclick="hideModal('editKeyModal')">&times;</button>
        </div>
        <input type="hidden" id="editKeyID">
        <div class="form-group">
            <label>API Key</label>
            <input type="text" id="editKeyValue" disabled>
        </div>
        <div class="form-group">
            <label>Server</label>
            <input type="text" id="editKeyServer" disabled>
        </div>
        <div class="form-group">
            <label>Status</label>
            <select id="editKeyStatus">
                <option value="enabled">Enabled</option>
                <option value="disabled">Disabled</option>
                <option value="auto">Auto</option>
            </select>
        </div>
        <div class="form-group">
            <label>Notes</label>
            <input type="text" id="editKeyNotes" placeholder="備註 / Notes">
        </div>
        <div class="btn-group">
            <button onclick="updateAPIKey()">儲存 Save</button>
            <button onclick="hideModal('editKeyModal')" style="background:#95a5a6">取消 Cancel</button>
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
            fetch('/api/settings').then(r=>r.json()).then(set=>{
                const resetHours = set.settings.weight_reset_hours || 4;
                const defaultMinutes = resetHours * 60;
                const now = Math.floor(Date.now() / 1000);
                tbody.innerHTML = d.server_api_keys.map(k=>{
                    const lastReset = k.last_reset_time || 0;
                    let remaining;
                    if (lastReset === 0) {
                        remaining = defaultMinutes;
                    } else {
                        const elapsed = now - lastReset;
                        const totalSeconds = resetHours * 3600;
                        remaining = Math.max(0, Math.floor((totalSeconds - elapsed) / 60));
                        if (remaining === 0) {
                            remaining = defaultMinutes;
                        }
                    }
                    const notTested = !k.last_check_time || k.last_check_time === 0 || k.last_check_time === "0" || k.last_check_time === "";
                    const checkOk = notTested || (k.last_check_result === true || k.last_check_result === "true" || k.last_check_result === "1" || k.last_check_result == true);
                    const statusLED = (k.status === 'disabled') ? '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:#888888;margin:0 auto;"></span>' : (checkOk ? '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:#0000ff;margin:0 auto;"></span>' : '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:#ff0000;margin:0 auto;"></span>');
                    return '<tr><td>'+(servers[k.server_id]||'')+'</td><td>'+(k.status||'enabled')+'</td><td>'+k.api_key+'</td><td>'+statusLED+'</td><td>'+(k.notes||'')+'</td><td>'+(k.negative_weight||0)+'</td><td>'+remaining+'</td><td class="actions"><button onclick="editAPIKey(\''+k.id+'\')">編輯 Edit</button> <button class="delete" onclick="deleteAPIKey(\''+k.id+'\')">刪除 Delete</button></td></tr>';
                }).join('');
            });
        });
    });
}
function addAPIKey() {
    fetch('/api/server-api-keys', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        server_id: document.getElementById('keyServerID').value,
        api_key: document.getElementById('keyValue').value,
        status: document.getElementById('keyStatus').value,
        notes: document.getElementById('keyNotes').value,
        is_active: true
    })}).then(r=>{if(r.ok){hideModal('addKeyModal');loadKeys();}else{r.json().then(d=>alert(d.error||'新增失敗 Add Failed'));}});
}
function deleteAPIKey(id) {
    if(confirm('確定要刪除? / Confirm delete?')) fetch('/api/server-api-keys/'+id,{method:'DELETE'}).then(loadKeys);
}
function testAllAPIKeys() {
    window.open('/test-results', '_blank');
}
function editAPIKey(id) {
    fetch('/api/server-api-keys').then(r=>r.json()).then(d=>{
        const key = d.server_api_keys.find(k=>k.id===id);
        fetch('/api/servers').then(r=>r.json()).then(s=>{
            const servers = {}; s.servers.forEach(x=>servers[x.id]=x.name);
            if(key) {
                document.getElementById('editKeyID').value = key.id;
                document.getElementById('editKeyValue').value = key.api_key;
                document.getElementById('editKeyServer').value = servers[key.server_id] || key.server_id;
                document.getElementById('editKeyStatus').value = key.status || 'enabled';
                document.getElementById('editKeyNotes').value = key.notes || '';
                showModal('editKeyModal');
            }
        });
    });
}
function updateAPIKey() {
    fetch('/api/server-api-keys', {method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        id: document.getElementById('editKeyID').value,
        status: document.getElementById('editKeyStatus').value,
        notes: document.getElementById('editKeyNotes').value
    })}).then(r=>{if(r.ok){hideModal('editKeyModal');loadKeys();}else{r.json().then(d=>alert(d.error||'儲存失敗 Save Failed'));}});
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

<div class="modal" id="editMapModal">
    <div class="modal-content">
        <div class="modal-header">
            <h3>編輯映射 Edit Mapping</h3>
            <button class="close" onclick="hideModal('editMapModal')">&times;</button>
        </div>
        <input type="hidden" id="editMapId">
        <div class="form-group">
            <label>選擇服務器模型 Select Server Model</label>
            <select id="editServerModelID">
                <option value="">請選擇模型 Select Model</option>
            </select>
        </div>
        <div class="form-group">
            <label>本地模型名稱 Local Model Name</label>
            <input type="text" id="editLocalModel">
        </div>
        <div class="btn-group">
            <button onclick="updateMapping()">儲存 Save</button>
            <button onclick="hideModal('editMapModal')" style="background:#95a5a6">取消 Cancel</button>
        </div>
    </div>
</div>
<script>
var availableModels = [];
function loadServerModels() {
    fetch('/api/server-models').then(r=>r.json()).then(d=>{
        availableModels = d.server_models;
        const sel = document.getElementById('serverModelID');
        sel.innerHTML = '<option value="">請選擇模型 Select Model</option>'+d.server_models.map(m=>'<option value="'+m.id+'">'+m.model_name+' ('+m.model_id+')</option>').join('');
        const editSel = document.getElementById('editServerModelID');
        editSel.innerHTML = '<option value="">請選擇模型 Select Model</option>'+d.server_models.map(m=>'<option value="'+m.id+'">'+m.model_name+' ('+m.model_id+')</option>').join('');
    });
}
function loadMaps() {
    fetch('/api/local-model-maps').then(r=>r.json()).then(d=>{
        const tbody = document.getElementById('mapsTable');
        fetch('/api/server-models').then(r=>r.json()).then(m=>{
            const models = {}; m.server_models.forEach(x=>models[x.id]=x.model_name);
            tbody.innerHTML = d.local_model_maps.map(x=>'<tr><td>'+x.local_model+'</td><td>'+(models[x.server_model_id]||'')+'</td><td class="actions"><button onclick="editMapping(\''+x.id+'\',\''+x.local_model+'\',\''+x.server_model_id+'\')">編輯 Edit</button><button class="delete" onclick="deleteMapping(\''+x.id+'\')">刪除 Delete</button></td></tr>').join('');
        });
    });
}
function addMapping() {
    fetch('/api/local-model-maps', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        local_model: document.getElementById('localModel').value,
        server_model_id: document.getElementById('serverModelID').value
    })}).then(r=>{if(r.ok){hideModal('addMapModal');loadMaps();}else{r.json().then(d=>alert(d.error||'新增失敗 Failed'));}});
}
function editMapping(id, localModel, serverModelId) {
    document.getElementById('editMapId').value = id;
    document.getElementById('editLocalModel').value = localModel;
    document.getElementById('editServerModelID').value = serverModelId;
    showModal('editMapModal');
}
function updateMapping() {
    fetch('/api/local-model-maps', {method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({
        id: document.getElementById('editMapId').value,
        local_model: document.getElementById('editLocalModel').value,
        server_model_id: document.getElementById('editServerModelID').value
    })}).then(r=>{if(r.ok){hideModal('editMapModal');loadMaps();}else{r.json().then(d=>alert(d.error||'儲存失敗 Save Failed'));}});
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
    <hr style="margin:20px 0;border:none;border-top:1px solid #eee;">
    <h3 style="font-size:16px;margin-bottom:15px;color:#2c3e50;">進階設置 Advanced Settings</h3>
    <div class="form-group">
        <label style="display:flex;align-items:center;gap:10px;">
            <input type="checkbox" id="enableNegativeWeight" style="width:auto;">
            啟用負權重模式 Enable Negative Weight Mode
        </label>
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">啟用後，錯誤次數多的 API Key 會被降低優先級</p>
    </div>
    <div class="form-group">
        <label>權重重置週期 (小時) Weight Reset Hours (2-8)</label>
        <input type="number" id="weightResetHours" min="2" max="8" value="4">
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">每多少小時將所有負權重重置為 0</p>
    </div>
    <div class="form-group">
        <label>4xx 錯誤權重值 4xx Weight</label>
        <input type="number" id="weight4xx" min="1" max="100" value="10">
    </div>
    <div class="form-group">
        <label>5xx 錯誤權重值 5xx Weight</label>
        <input type="number" id="weight5xx" min="1" max="100" value="50">
    </div>
    <hr style="margin:20px 0;border:none;border-top:1px solid #eee;">
    <h3 style="font-size:16px;margin-bottom:15px;color:#2c3e50;">分離式超時設置 Separate Timeout Settings</h3>
    <div class="form-group">
        <label>連接超時 Connect Timeout (秒) (1-30)</label>
        <input type="number" id="connectTimeout" min="1" max="30" value="10">
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">TCP 連接建立超時 (預設 10 秒)</p>
    </div>
    <div class="form-group">
        <label>超時權重 Timeout Weight</label>
        <input type="number" id="timeoutWeight" min="1" max="100" value="30">
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">讀取響應超時時的權重 (預設 30)</p>
    </div>
    <div class="form-group">
        <label>連接超時權重 Connect Timeout Weight</label>
        <input type="number" id="connectTimeoutWeight" min="1" max="100" value="10">
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">連接超時時的權重 (預設 10)</p>
    </div>
    <div class="form-group">
        <label style="display:flex;align-items:center;gap:10px;">
            <input type="checkbox" id="enableRetryOnTimeout" style="width:auto;">
            超時時也重試 Retry on Timeout
        </label>
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">超時時是否啟用重試 (默認關閉)</p>
    </div>
    <hr style="margin:20px 0;border:none;border-top:1px solid #eee;">
    <div class="form-group">
        <label style="display:flex;align-items:center;gap:10px;">
            <input type="checkbox" id="enableRetry" style="width:auto;">
            啟用錯誤重試 Enable Error Retry
        </label>
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">當 API Key 發生錯誤時，自動切換到其他 Key 重試</p>
    </div>
    <div class="form-group">
        <label>最大重試次數 Max Retries (1-5)</label>
        <input type="number" id="maxRetries" min="1" max="5" value="3">
    </div>
    <hr style="margin:20px 0;border:none;border-top:1px solid #eee;">
    <h3 style="font-size:16px;margin-bottom:15px;color:#2c3e50;">權重管理 Weight Management</h3>
    <div style="margin-bottom:20px;">
        <button onclick="resetAllWeights()">重置所有負權重 Reset All Weights</button>
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">將所有伺服器的 API Key 權重重置為 0</p>
    </div>
    <hr style="margin:20px 0;border:none;border-top:1px solid #eee;">
    <h3 style="font-size:16px;margin-bottom:15px;color:#2c3e50;">API Key 健康監控 API Key Health Monitoring</h3>
    <div class="form-group">
        <label style="color:#27ae60;font-weight:bold;">狀態：已啟用 Status: Enabled</label>
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">自動定時檢查狀態為 "auto" 的 API Key 是否正常工作，無關閉功能</p>
    </div>
    <div class="form-group">
        <label>自動檢查間隔 (小時) Auto Check Interval (Hours) (3-12)</label>
        <input type="number" id="autoCheckIntervalHours" min="3" max="12" value="6">
        <p style="color:#7f8c8d;font-size:12px;margin-top:5px;">定時檢查 API Key 的間隔 (預設 6 小時)</p>
    </div>
    <div style="margin-top:20px;">
        <button onclick="saveSettings()">儲存設定 Save Settings</button>
        <button onclick="restartSystem()">重啟 llm_gateway Restart</button>
    </div>
    <div style="color:#7f8c8d;margin-top:15px;line-height:1.8;">
        <h4 style="color:#2c3e50;margin-bottom:8px;">API Key 輪換模式說明 / API Key Rotation Mode</h4>
        <ul style="padding-left:20px;">
            <li><strong>輪詢模式 (預設) / Round-robin (Default)</strong>：關閉「負權重模式」時使用。按順序輪流使用 Key (A→B→C→A→B→...)。<br>When disabled, uses keys in sequence (A→B→C→A→B→...).</li>
            <li><strong>負權重模式 / Negative Weight Mode</strong>：開啟「啟用負權重模式」時使用。選擇當前權重最低的 Key，錯誤時增加權重。權重每 N 小時重置。可避免問題 Key 持續被使用。<br>When enabled, selects lowest weight key. Error increases weight. Weight resets every N hours.</li>
            <li><strong>錯誤重試 / Error Retry</strong>：開啟「啟用錯誤重試」時使用。Key 失敗時自動切換到下一個 Key 重試，最多 N 次。4xx 立即切換，5xx 持續重試。<br>When enabled, auto-switches to next key on error. Max N retries. 4xx switches immediately, 5xx retries.</li>
        </ul>
    </div>
</div>
<script>
fetch('/api/settings').then(r=>r.json()).then(d=>{
    if(d.settings) {
        document.getElementById('timeout').value = d.settings.timeout || 5;
        document.getElementById('enableNegativeWeight').checked = d.settings.enable_negative_weight || false;
        document.getElementById('weightResetHours').value = d.settings.weight_reset_hours || 4;
        document.getElementById('weight4xx').value = d.settings.weight_4xx || 10;
        document.getElementById('weight5xx').value = d.settings.weight_5xx || 50;
        
        document.getElementById('connectTimeout').value = d.settings.connect_timeout || 10;
        document.getElementById('timeoutWeight').value = d.settings.timeout_weight || 30;
        document.getElementById('connectTimeoutWeight').value = d.settings.connect_timeout_weight || 10;
        document.getElementById('enableRetryOnTimeout').checked = d.settings.enable_retry_on_timeout || false;
        
        document.getElementById('enableRetry').checked = d.settings.enable_retry || false;
        document.getElementById('maxRetries').value = d.settings.max_retries || 3;
        
        document.getElementById('autoCheckIntervalHours').value = d.settings.auto_check_interval_hours || 6;
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
    
    var enableNegativeWeight = document.getElementById('enableNegativeWeight').checked;
    var weightResetHours = parseInt(document.getElementById('weightResetHours').value);
    var weight4xx = parseInt(document.getElementById('weight4xx').value);
    var weight5xx = parseInt(document.getElementById('weight5xx').value);
    
    var connectTimeout = parseInt(document.getElementById('connectTimeout').value);
    var timeoutWeight = parseInt(document.getElementById('timeoutWeight').value);
    var connectTimeoutWeight = parseInt(document.getElementById('connectTimeoutWeight').value);
    var enableRetryOnTimeout = document.getElementById('enableRetryOnTimeout').checked;
    
    var enableRetry = document.getElementById('enableRetry').checked;
    var maxRetries = parseInt(document.getElementById('maxRetries').value);
    
    var autoCheckIntervalHours = parseInt(document.getElementById('autoCheckIntervalHours').value);
    
    fetch('/api/settings', {
        method:'POST',
        headers:{'Content-Type':'application/json'},
        body:JSON.stringify({
            timeout: timeout,
            enable_negative_weight: enableNegativeWeight,
            weight_reset_hours: weightResetHours,
            weight_4xx: weight4xx,
            weight_5xx: weight5xx,
            connect_timeout: connectTimeout,
            timeout_weight: timeoutWeight,
            connect_timeout_weight: connectTimeoutWeight,
            enable_retry_on_timeout: enableRetryOnTimeout,
            enable_retry: enableRetry,
            max_retries: maxRetries,
            enable_auto_check_api_key: true,
            auto_check_interval_hours: autoCheckIntervalHours
        })
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
function resetAllWeights() {
    if(confirm('確定要重置所有負權重? / Confirm reset all weights?')) {
        fetch('/api/reset-weights', {method:'POST'}).then(r=>r.json()).then(d=>{
            alert(d.message || '完成 Done');
            loadKeys();
        });
    }
}
</script>
`

const TestResultPage = `
<div class="card">
    <h2>API Key 測試結果 Test Results</h2>
    <p>測試進行中... Testing in progress...</p>
    <button onclick="location.reload()">重新整理 Refresh</button>
    <button onclick="location.href='/'">返回 Home</button>
</div>
<div class="card">
    <table>
        <thead>
            <tr>
                <th>Server</th>
                <th>API Key</th>
                <th>Status</th>
                <th>Result</th>
                <th>HTTP Status</th>
                <th>Time</th>
            </tr>
        </thead>
        <tbody id="testResults"></tbody>
    </table>
</div>
<script>
fetch('/api/test-key').then(r=>r.json()).then(d=>{
    const tbody = document.getElementById('testResults');
    if(!d.results || d.results.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;color:#ff0000;">無 API Key 可測試 No API Keys to test</td></tr>';
        return;
    }
    fetch('/api/servers').then(r=>r.json()).then(s=>{
        const servers = {}; s.servers.forEach(x=>servers[x.id]=x.name);
        tbody.innerHTML = d.results.map(r=>{
            const resultLED = r.success ? '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:#0000ff;margin:0 auto;"></span>' : '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:#ff0000;margin:0 auto;"></span>';
            const maskedKey = r.api_key ? (r.api_key.substring(0, 6) + '****' + r.api_key.substring(r.api_key.length - 4)) : '';
            return '<tr><td>'+(servers[r.server_id]||'')+'</td><td>'+maskedKey+'</td><td>'+(r.status||'')+'</td><td>'+resultLED+'</td><td>'+(r.http_status||'')+'</td><td>'+(r.timestamp ? new Date(r.timestamp*1000).toLocaleString() : '')+'</td></tr>';
        }).join('');
    });
});
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
