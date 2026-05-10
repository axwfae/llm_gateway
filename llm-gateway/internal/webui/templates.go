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
        *{margin:0;padding:0;box-sizing:border-box}
        body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;display:flex;height:100vh;background:#f5f5f5}
        .sidebar{width:220px;background:#2c3e50;color:#fff;padding:20px;display:flex;flex-direction:column}
        .sidebar h1{font-size:18px;margin-bottom:30px;padding-bottom:10px;border-bottom:1px solid #34495e}
        .sidebar nav a{display:block;padding:10px 15px;color:#bdc3c7;text-decoration:none;border-radius:5px;margin-bottom:3px;transition:all .2s;font-size:14px}
        .sidebar nav a:hover,.sidebar nav a.active{background:#34495e;color:#fff}
        .main{flex:1;padding:25px;overflow-y:auto}
        .card{background:#fff;border-radius:8px;padding:20px;margin-bottom:20px;box-shadow:0 2px 4px rgba(0,0,0,.1)}
        .card h2{font-size:18px;margin-bottom:15px;color:#2c3e50}
        .form-group{margin-bottom:15px}
        .form-group label{display:block;margin-bottom:5px;font-weight:500;color:#555;font-size:13px}
        .form-group input,.form-group select{width:100%;padding:8px 10px;border:1px solid #ddd;border-radius:5px;font-size:14px}
        .form-group input:focus,.form-group select:focus{outline:0;border-color:#3498db}
        .form-row{display:flex;gap:15px}
        .form-row .form-group{flex:1}
        .form-group.checkbox label{display:flex;align-items:center;gap:8px;cursor:pointer}
        .form-group.checkbox input[type=checkbox]{width:auto}
        button{padding:8px 16px;background:#3498db;color:#fff;border:none;border-radius:5px;cursor:pointer;font-size:13px}
        button:hover{background:#2980b9}
        button.danger{background:#e74c3c}
        button.danger:hover{background:#c0392b}
        button.success{background:#2ecc71}
        button.success:hover{background:#27ae60}
        button.warning{background:#f39c12}
        button.warning:hover{background:#e67e22}
        button.small{padding:4px 10px;font-size:12px}
        button:disabled{opacity:.5;cursor:not-allowed}
        table{width:100%;border-collapse:collapse;font-size:13px}
        th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #eee}
        th{background:#f8f9fa;font-weight:600;color:#555;white-space:nowrap}
        tr:hover{background:#f8f9fa}
        .actions{display:flex;gap:6px;flex-wrap:wrap}
        .modal{display:none;position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,.5);z-index:1000}
        .modal.active{display:flex;align-items:center;justify-content:center}
        .modal-content{background:#fff;padding:25px;border-radius:8px;width:500px;max-width:95%;max-height:85vh;overflow-y:auto}
        .modal-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:20px}
        .modal-header h3{font-size:18px}
        .close{background:0 0;border:none;font-size:24px;cursor:pointer;color:#999;padding:0;line-height:1}
        .close:hover{color:#333}
        .led{display:inline-block;width:12px;height:12px;border-radius:50%;margin-right:5px;vertical-align:middle}
        .led-blue{background:#3498db}
        .led-red{background:#e74c3c}
        .led-gray{background:#bdc3c7}
        button.purple{background:#8e44ad}
        button.purple:hover{background:#7d3c98}
        .grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(230px,1fr));gap:20px}
        .stat-card{background:#fff;border-radius:8px;padding:25px;text-align:center;box-shadow:0 2px 4px rgba(0,0,0,.1);cursor:pointer;transition:transform .2s}
        .stat-card:hover{transform:translateY(-2px)}
        .stat-card .number{font-size:36px;font-weight:700;color:#2c3e50}
        .stat-card .label{font-size:14px;color:#7f8c8d;margin-top:5px}
        .stat-card .icon{font-size:32px;margin-bottom:10px}
        .badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:11px;font-weight:500}
        .badge-green{background:#d4edda;color:#155724}
        .badge-red{background:#f8d7da;color:#721c24}
.badge-yellow{background:#fff3cd;color:#856404}
.badge-blue{background:#d1ecf1;color:#0c5460}
.badge-gray{background:#e2e3e5;color:#383d41}
        .toolbar{display:flex;gap:10px;margin-bottom:15px;flex-wrap:wrap;align-items:center}
        hr{border:none;border-top:1px solid #eee;margin:20px 0}
        .help-text{color:#7f8c8d;font-size:12px;margin-top:5px}
        .inline-result{margin-top:8px;padding:8px 12px;border-radius:5px;font-size:13px}
        .inline-result.ok{background:#d4edda;color:#155724}
        .inline-result.fail{background:#f8d7da;color:#721c24}
        .inline-result.loading{background:#fff3cd;color:#856404}
    </style>
</head>
<body>
    <div class="sidebar">
        <h1>LLM Gateway</h1>
        <nav>
            <a href="/" class="{{if eq .Title "首頁 / Home"}}active{{end}}">🏠 <span>首頁 Home</span></a>
            <a href="/servers" class="{{if eq .Title "服務器設置 / Servers"}}active{{end}}">🖥️ <span>服務器 Servers</span></a>
            <a href="/server-models" class="{{if eq .Title "模型設置 / Models"}}active{{end}}">📋 <span>模型 Models</span></a>
            <a href="/api-keys" class="{{if eq .Title "API Key 設置 / API Keys"}}active{{end}}">🔑 <span>API Keys</span></a>
            <a href="/pending-pool" class="{{if eq .Title "待選池 / Pending Pool"}}active{{end}}">⏳ <span>待選池 Pool</span></a>
            <a href="/local-models" class="{{if eq .Title "本地模型映射 / Local Models"}}active{{end}}">🔗 <span>映射 Mapping</span></a>
            <a href="/settings" class="{{if eq .Title "系統設置 / Settings"}}active{{end}}">⚙️ <span>設置 Settings</span></a>
        </nav>
    </div>
    <div class="main">{{.Content}}</div>
    <script>
    function showModal(id){document.getElementById(id).classList.add('active')}
    function hideModal(id){document.getElementById(id).classList.remove('active')}
function escapeHtml(s){if(!s)return '';return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
    function ledHtml(r){if(!r||r=='')return '<span class="led led-gray"></span>';if(r=='ok'||r=='success')return '<span class="led led-blue"></span>';return '<span class="led led-red"></span>'}
    function modeText(s){if(s=='auto')return '自動 Auto';if(s=='enabled')return '開啟 Enabled';return '關閉 Disabled'}
    function statusText(r){if(!r||r=='')return '未測試';if(r=='ok'||r=='success')return '通過 Pass';return '失敗 Fail'}
    </script>
</body>
</html>
{{end}}
`

const IndexPage = `
<div class="grid">
    <div class="stat-card" onclick="location.href='/servers'">
        <div class="icon">🖥️</div>
        <div class="number" id="statServers">-</div>
        <div class="label">服務器 Servers</div>
    </div>
    <div class="stat-card" onclick="location.href='/server-models'">
        <div class="icon">📋</div>
        <div class="number" id="statModels">-</div>
        <div class="label">模型 Models</div>
    </div>
    <div class="stat-card" onclick="location.href='/api-keys'">
        <div class="icon">🔑</div>
        <div class="number" id="statKeys">-</div>
        <div class="label">API Keys</div>
    </div>
    <div class="stat-card" onclick="location.href='/local-models'">
        <div class="icon">🔗</div>
        <div class="number" id="statMaps">-</div>
        <div class="label">映射 Mappings</div>
    </div>
</div>
<div class="card">
    <h2>快速開始 Quick Start</h2>
    <ol style="margin-left:20px;line-height:2;font-size:14px">
        <li>在 <a href="/servers">服務器 Servers</a> 頁面新增 LLM 服務器</li>
        <li>在 <a href="/server-models">模型 Models</a> 頁面新增每個服務器支援的模型</li>
        <li>在 <a href="/api-keys">API Keys</a> 頁面新增 API Key（也可先加入<a href="/pending-pool">待選池 Pending Pool</a>測試）</li>
        <li>在 <a href="/local-models">映射 Mapping</a> 頁面設定本地模型名稱到服務器模型的映射</li>
        <li>透過 API Port (<code>:18869</code>) 調用 <code>/v1/chat/completions</code></li>
    </ol>
</div>
<div class="card">
    <h2>系統狀態 System Status</h2>
    <p>版本 Version: <span id="appVersion">-</span></p>
    <p>API Proxy Port 18869: <span id="proxyStatus" class="badge badge-yellow">檢查中...</span></p>
    <p>Web UI Port 18866: <span class="badge badge-green">運行中 Running</span></p>
</div>
<script>
function loadVersion(){fetch('/api/version').then(r=>r.json()).then(function(d){document.getElementById('appVersion').textContent='v'+d.version;}).catch(function(){})}
loadVersion();
Promise.all([
    fetch('/api/servers').then(r=>r.json()).then(d=>document.getElementById('statServers').textContent=(d.servers||[]).length).catch(()=>{}),
    fetch('/api/server-models').then(r=>r.json()).then(d=>document.getElementById('statModels').textContent=(d.server_models||[]).length).catch(()=>{}),
    fetch('/api/server-api-keys').then(r=>r.json()).then(d=>document.getElementById('statKeys').textContent=(d.server_api_keys||[]).length).catch(()=>{}),
    fetch('/api/local-model-maps').then(r=>r.json()).then(d=>document.getElementById('statMaps').textContent=(d.local_model_maps||[]).length).catch(()=>{})
]).then(()=>{
    document.getElementById('proxyStatus').textContent='運行中 Running';
    document.getElementById('proxyStatus').className='badge badge-green';
}).catch(()=>{
    document.getElementById('proxyStatus').textContent='無法連接 Error';
    document.getElementById('proxyStatus').className='badge badge-red';
});
</script>
`

const ServersPage = `
<div class="card">
    <h2>服務器設置 Server Settings</h2>
    <button onclick="showModal('addServerModal')">新增服務器 Add Server</button>
</div>
<div class="card">
    <table><thead><tr>
        <th>名稱 Name</th><th>API URL</th><th>API 類型 Type</th><th>操作 Actions</th>
    </tr></thead>
    <tbody id="serversTable"></tbody></table>
</div>
<div class="modal" id="addServerModal">
    <div class="modal-content">
        <div class="modal-header"><h3>新增服務器 Add Server</h3><button class="close" onclick="hideModal('addServerModal')">&times;</button></div>
        <div class="form-group"><label>服務器名稱 Server Name</label><input type="text" id="serverName" placeholder="e.g. OpenAI"></div>
        <div class="form-group"><label>API URL</label><input type="text" id="serverAPIURL" placeholder="e.g. https://api.openai.com/v1"></div>
        <div class="form-group"><label>API 類型 API Type</label>
            <select id="serverAPIType"><option value="openai">OpenAI</option><option value="anthropic">Anthropic</option><option value="deepseek">DeepSeek</option><option value="ollama">Ollama</option><option value="other">Other</option></select>
        </div>
        <button onclick="addServer()">新增 Add</button>
        <button onclick="hideModal('addServerModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<div class="modal" id="editServerModal">
    <div class="modal-content">
        <div class="modal-header"><h3>編輯服務器 Edit Server</h3><button class="close" onclick="hideModal('editServerModal')">&times;</button></div>
        <input type="hidden" id="editServerId">
        <div class="form-group"><label>服務器名稱 Server Name</label><input type="text" id="editServerName"></div>
        <div class="form-group"><label>API URL</label><input type="text" id="editServerAPIURL"></div>
        <div class="form-group"><label>API 類型 API Type</label>
            <select id="editServerAPIType"><option value="openai">OpenAI</option><option value="anthropic">Anthropic</option><option value="deepseek">DeepSeek</option><option value="ollama">Ollama</option><option value="other">Other</option></select>
        </div>
        <button onclick="updateServer()">儲存 Save</button>
        <button onclick="hideModal('editServerModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<script>
function loadServers(){fetch('/api/servers').then(r=>r.json()).then(d=>{
    document.getElementById('serversTable').innerHTML=d.servers.map(s=>'<tr><td>'+escapeHtml(s.name)+'</td><td>'+escapeHtml(s.api_url)+'</td><td>'+s.api_type+'</td><td class=actions><button class=small onclick="editServer(\''+s.id+'\',\''+escapeHtml(s.name).replace(/'/g,"\\'")+'\',\''+escapeHtml(s.api_url).replace(/'/g,"\\'")+'\',\''+s.api_type+'\')">編輯 Edit</button><button class="small danger" onclick="deleteServer(\''+s.id+'\')">刪除 Delete</button></td></tr>').join('')
})}
function addServer(){fetch('/api/servers',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:document.getElementById('serverName').value,api_url:document.getElementById('serverAPIURL').value,api_type:document.getElementById('serverAPIType').value})}).then(r=>{if(r.ok){hideModal('addServerModal');document.getElementById('serverName').value='';document.getElementById('serverAPIURL').value='';loadServers()}else{r.json().then(d=>alert(d.error))}})}
function editServer(id,n,u,ty){document.getElementById('editServerId').value=id;document.getElementById('editServerName').value=n;document.getElementById('editServerAPIURL').value=u;document.getElementById('editServerAPIType').value=ty;showModal('editServerModal')}
function updateServer(){fetch('/api/servers',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:document.getElementById('editServerId').value,name:document.getElementById('editServerName').value,api_url:document.getElementById('editServerAPIURL').value,api_type:document.getElementById('editServerAPIType').value})}).then(r=>{if(r.ok){hideModal('editServerModal');loadServers()}else{r.json().then(d=>alert(d.error))}})}
function deleteServer(id){if(confirm('確定要刪除? Confirm delete?'))fetch('/api/servers/'+id,{method:'DELETE'}).then(r=>{if(r.ok)loadServers();else r.json().then(d=>alert(d.error))})}
loadServers();
</script>
`

const ServerModelsPage = `
<div class="card">
    <h2>服務器模型設置 Server Models</h2>
    <button onclick="showModal('addModelModal')">新增模型 Add Model</button>
</div>
<div class="card">
    <table><thead><tr>
        <th>模型名稱 Model Name</th><th>模型 ID Model ID</th><th>關聯服務器 Server</th><th>操作 Actions</th>
    </tr></thead>
    <tbody id="modelsTable"></tbody></table>
</div>
<div class="modal" id="addModelModal">
    <div class="modal-content">
        <div class="modal-header"><h3>新增模型 Add Model</h3><button class="close" onclick="hideModal('addModelModal')">&times;</button></div>
        <div class="form-group"><label>選擇服務器 Select Server</label><select id="modelServerID"><option value="">請選擇 Select</option></select></div>
        <div class="form-group"><label>模型名稱 Model Name</label><input type="text" id="modelName" placeholder="e.g. GPT-4"></div>
        <div class="form-group"><label>模型 ID Model ID (API)</label><input type="text" id="modelID" placeholder="e.g. gpt-4"></div>
        <button onclick="addServerModel()">新增 Add</button>
        <button onclick="hideModal('addModelModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<div class="modal" id="editModelModal">
    <div class="modal-content">
        <div class="modal-header"><h3>編輯模型 Edit Model</h3><button class="close" onclick="hideModal('editModelModal')">&times;</button></div>
        <input type="hidden" id="editModelId">
        <div class="form-group"><label>選擇服務器 Select Server</label><select id="editModelServerID"><option value="">請選擇 Select</option></select></div>
        <div class="form-group"><label>模型名稱 Model Name</label><input type="text" id="editModelName"></div>
        <div class="form-group"><label>模型 ID Model ID</label><input type="text" id="editModelID"></div>
        <button onclick="updateServerModel()">儲存 Save</button>
        <button onclick="hideModal('editModelModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<script>
var servers = [];
function loadServersList(){fetch('/api/servers').then(r=>r.json()).then(d=>{
    servers=d.servers||[];var h='<option value="">請選擇 Select</option>';
    servers.forEach(s=>{h+='<option value="'+s.id+'">'+escapeHtml(s.name)+'</option>'});
    document.getElementById('modelServerID').innerHTML=h;
    document.getElementById('editModelServerID').innerHTML=h;
    loadServerModels()
})}
function loadServerModels(){fetch('/api/server-models').then(r=>r.json()).then(d=>{
    document.getElementById('modelsTable').innerHTML=(d.server_models||[]).map(m=>{
        var sn=servers.find(s=>s.id==m.server_id);return '<tr><td>'+escapeHtml(m.model_name)+'</td><td>'+escapeHtml(m.model_id)+'</td><td>'+(sn?escapeHtml(sn.name):m.server_id)+'</td><td class=actions><button class=small onclick="editModel(\''+m.id+'\')">編輯 Edit</button><button class="small danger" onclick="deleteModel(\''+m.id+'\')">刪除 Delete</button></td></tr>'
    }).join('')
})}
function addServerModel(){
    var sid=document.getElementById('modelServerID').value;
    if(!sid){alert('請選擇服務器 / Please select a server');return}
    fetch('/api/server-models',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({server_id:sid,model_name:document.getElementById('modelName').value,model_id:document.getElementById('modelID').value})}).then(r=>{if(r.ok){hideModal('addModelModal');loadServerModels();document.getElementById('modelName').value='';document.getElementById('modelID').value=''}else{r.json().then(d=>alert(d.error))}})}
function editModel(id){fetch('/api/server-models').then(r=>r.json()).then(d=>{
    var m=(d.server_models||[]).find(x=>x.id==id);if(!m)return;
    document.getElementById('editModelId').value=m.id;
    document.getElementById('editModelServerID').value=m.server_id;
    document.getElementById('editModelName').value=m.model_name;
    document.getElementById('editModelID').value=m.model_id;
    showModal('editModelModal')
})}
function updateServerModel(){fetch('/api/server-models',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:document.getElementById('editModelId').value,server_id:document.getElementById('editModelServerID').value,model_name:document.getElementById('editModelName').value,model_id:document.getElementById('editModelID').value})}).then(r=>{if(r.ok){hideModal('editModelModal');loadServerModels()}else{r.json().then(d=>alert(d.error))}})}
function deleteModel(id){if(confirm('確定要刪除? Confirm delete?'))fetch('/api/server-models/'+id,{method:'DELETE'}).then(r=>{if(r.ok)loadServerModels();else r.json().then(d=>alert(d.error))})}
loadServersList();
</script>
`

const APIKeysPage = `
<div class="card">
    <h2>API Key 設置 API Keys</h2>
    <button onclick="showModal('addKeyModal')">新增 API Key</button>
</div>
<div class="card">
    <table><thead><tr>
        <th>API Key</th><th>模式 Mode</th><th>狀態 Status</th><th>關聯服務器 Server</th><th>權重 Weight</th><th>待選池 Pool</th><th>備註 Notes</th><th>操作 Actions</th>
    </tr></thead>
    <tbody id="keysTable"></tbody></table>
</div>
<div class="modal" id="addKeyModal">
    <div class="modal-content">
        <div class="modal-header"><h3>新增 API Key</h3><button class="close" onclick="hideModal('addKeyModal')">&times;</button></div>
        <div class="form-group"><label>選擇服務器 Select Server</label><select id="keyServerID"><option value="">請選擇 Select</option></select></div>
        <div class="form-group"><label>API Key</label><input type="text" id="keyValue" placeholder="Enter API Key"></div>
        <div class="form-group"><label>工作模式 Mode</label><select id="keyStatus"><option value="auto">自動 Auto</option><option value="enabled">開啟 Enabled</option><option value="disabled">關閉 Disabled</option></select></div>
        <div class="form-group"><label>備註 Notes</label><input type="text" id="keyNotes" placeholder="備註 Notes"></div>
        <button onclick="addAPIKey()">新增 Add</button>
        <button onclick="hideModal('addKeyModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<div class="modal" id="editKeyModal">
    <div class="modal-content">
        <div class="modal-header"><h3>編輯 API Key</h3><button class="close" onclick="hideModal('editKeyModal')">&times;</button></div>
        <input type="hidden" id="editKeyID">
        <div class="form-group"><label>API Key（建立後不可修改 / Immutable after creation）</label><input type="text" id="editKeyValue" disabled></div>
        <div class="form-group"><label>服務器 Server（建立後不可修改 / Immutable after creation）</label><input type="text" id="editKeyServerDisplay" disabled></div>
        <div class="form-group"><label>工作模式 Mode</label><select id="editKeyStatus"><option value="auto">自動 Auto</option><option value="enabled">開啟 Enabled</option><option value="disabled">關閉 Disabled</option></select></div>
        <div class="form-group"><label>備註 Notes</label><input type="text" id="editKeyNotes"></div>
        <button onclick="updateAPIKey()">儲存 Save</button>
        <button onclick="hideModal('editKeyModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<script>
var servers=[];
function loadServers(){fetch('/api/servers').then(r=>r.json()).then(d=>{
    servers=d.servers||[];var h='<option value="">請選擇 Select</option>';
    servers.forEach(s=>{h+='<option value="'+s.id+'">'+escapeHtml(s.name)+'</option>'});
    document.getElementById('keyServerID').innerHTML=h;
    loadAPIKeys()
})}
loadServers();
function loadAPIKeys(){fetch('/api/server-api-keys').then(r=>r.json()).then(d=>{
    document.getElementById('keysTable').innerHTML=(d.server_api_keys||[]).map(k=>{
        var sn=servers.find(s=>s.id==k.server_id);
        var rem='';if(k.weight_reset_hours&&k.last_reset_time){var elapsed=Math.floor((Date.now()-new Date(k.last_reset_time).getTime())/60000);rem=Math.max(0,k.weight_reset_hours*60-elapsed)+'m'}
        return '<tr><td>'+escapeHtml(k.api_key||'')+'</td><td>'+modeText(k.status)+'</td><td>'+ledHtml(k.last_check_result)+statusText(k.last_check_result)+(k.last_check_duration?' <span style="color:#7f8c8d;font-size:11px">'+escapeHtml(k.last_check_duration)+'</span>':'')+'</td><td>'+(sn?escapeHtml(sn.name):k.server_id)+'</td><td>'+(k.negative_weight||0)+'</td><td>'+(k.pending_pool?'<span class="badge badge-blue">Pool</span>':(k.status=='disabled'?'<span class="badge badge-gray">已停用</span>':'<span class="badge badge-yellow">待測試</span>'))+'</td><td>'+escapeHtml(k.notes||'')+'</td><td class=actions><button class=small onclick="editKey(\''+k.id+'\')">編輯 Edit</button><button class="small danger" onclick="deleteKey(\''+k.id+'\')">刪除 Del</button></td></tr>'
    }).join('')
})}
function addAPIKey(){
    var sid=document.getElementById('keyServerID').value;
    if(!sid){alert('請選擇服務器 / Please select a server');return}
    fetch('/api/server-api-keys',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({server_id:sid,api_key:document.getElementById('keyValue').value,status:document.getElementById('keyStatus').value,notes:document.getElementById('keyNotes').value})}).then(r=>{if(r.ok){hideModal('addKeyModal');document.getElementById('keyValue').value='';document.getElementById('keyNotes').value='';loadAPIKeys()}else{r.json().then(d=>alert(d.error))}})}
function editKey(id){fetch('/api/server-api-keys').then(r=>r.json()).then(d=>{
    var k=(d.server_api_keys||[]).find(x=>x.id==id);if(!k)return;
    var sn=servers.find(s=>s.id==k.server_id);
    document.getElementById('editKeyID').value=k.id;document.getElementById('editKeyValue').value=k.api_key;document.getElementById('editKeyServerDisplay').value=sn?sn.name:k.server_id;document.getElementById('editKeyStatus').value=k.status;document.getElementById('editKeyNotes').value=k.notes||'';showModal('editKeyModal')
})}
function updateAPIKey(){fetch('/api/server-api-keys',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:document.getElementById('editKeyID').value,status:document.getElementById('editKeyStatus').value,notes:document.getElementById('editKeyNotes').value})}).then(r=>{if(r.ok){hideModal('editKeyModal');loadAPIKeys()}else{r.json().then(d=>alert(d.error))}})}
function deleteKey(id){if(confirm('確定要刪除? Confirm delete?'))fetch('/api/server-api-keys/'+id,{method:'DELETE'}).then(r=>{if(r.ok)loadAPIKeys();else r.json().then(d=>alert(d.error))})}
</script>
`

const PendingPoolPage = `
<div class="card">
    <h2>待選池 Pending Pool</h2>
    <p style="color:#7f8c8d;font-size:13px">根據 API Key 的工作模式自動管理。可在下方選擇服務器進行批量測試</p>
    <div class="toolbar" style="margin-top:10px">
        <select id="poolTestServerSelect" class="server-select"><option value="">選擇服務器 Select Server</option></select>
        <button onclick="testAllPoolKeys()">測試所選服務器所有 Key Test All</button>
    </div>
</div>
<div id="poolContainers"></div>
<div class="card">
    <h2>最近測試記錄 Recent Test History</h2>
    <div class="toolbar">
        <button onclick="loadHistory()" class="small">重新整理 Refresh</button>
        <label style="display:flex;align-items:center;gap:5px;font-size:13px">
            <input type="checkbox" id="autoRefresh" style="width:auto" onchange="toggleAutoRefresh()"> 自動刷新 Auto Refresh
        </label>
    </div>
    <table><thead><tr>
        <th>API Key</th><th>狀態 Status</th><th>耗時 Duration</th><th>錯誤 Error</th>
    </tr></thead>
    <tbody id="historyTable"></tbody></table>
</div>
<script>
function loadPool(){fetch('/api/pending-pool').then(r=>r.json()).then(d=>{
    var html='';
    var serverKeys=Object.keys(d.servers||{});
    if(serverKeys.length===0){document.getElementById('poolContainers').innerHTML='<div class="card"><p style="color:#7f8c8d;text-align:center">目前沒有 Key 在待選池中 / No keys in pending pool</p></div>';return}
    serverKeys.forEach(function(sid){
        var keys=d.servers[sid];
        var sn=servers.find(s=>s.id==sid);
        html+='<div class="card"><h2>'+(sn?escapeHtml(sn.name):sid)+'</h2><table><thead><tr><th>操作 Actions</th><th>API Key</th><th>模式 Mode</th><th>狀態 Status</th><th>最後測試 Last Test</th><th>測試結果 Result</th></tr></thead><tbody>';
        keys.forEach(function(k){
            var errMsg='';if(k.last_check_result&&k.last_check_result!='ok'&&k.last_check_result!='success'&&k.last_check_duration){errMsg=k.last_check_result}
            html+='<tr><td><button class="small purple" onclick="testSinglePoolKey(\''+k.id+'\',this)">測試 Test</button></td><td>'+escapeHtml(k.api_key)+'</td><td>'+modeText(k.status)+'</td><td>'+ledHtml(k.last_check_result)+statusText(k.last_check_result)+(k.last_check_duration?' <span style="color:#7f8c8d;font-size:11px">'+escapeHtml(k.last_check_duration)+'</span>':'')+'</td><td>'+(k.last_check_time?new Date(k.last_check_time).toLocaleString():'')+'</td><td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;font-size:12px;color:#7f8c8d">'+(errMsg?escapeHtml(errMsg):'')+'</td></tr>'
        });
        html+='</tbody></table></div>'
    });
    document.getElementById('poolContainers').innerHTML=html
})}
function testSinglePoolKey(id,btn){btn.disabled=true;btn.textContent='測試中...';fetch('/api/test-key',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({key_id:id})}).then(function(){btn.disabled=false;btn.textContent='重新測試 Retest';loadPool();loadHistory()})}
function testAllPoolKeys(){var sid=document.getElementById('poolTestServerSelect').value;if(!sid){alert('請選擇服務器 Please select a server');return}
    var btn=document.querySelector('#poolTestServerSelect + button');btn.disabled=true;btn.textContent='測試中...';
    fetch('/api/test-key?server_id='+sid).then(function(){btn.disabled=false;btn.textContent='測試所選服務器所有 Key Test All';loadPool();loadHistory()})
}
var autoRefreshTimer=null;
function loadHistory(){fetch('/api/test-results').then(r=>r.json()).then(d=>{
    document.getElementById('historyTable').innerHTML=(d.results||[]).map(function(r){
        return '<tr><td>'+escapeHtml(r.key_masked||'')+'</td><td>'+ledHtml(r.status)+' '+(r.status=='ok'||r.status=='success'?'通過 Pass':'失敗 Fail')+'</td><td>'+escapeHtml(r.duration||'')+'</td><td>'+escapeHtml(r.error||'')+'</td></tr>'
    }).join('')
})}
function toggleAutoRefresh(){if(document.getElementById('autoRefresh').checked){autoRefreshTimer=setInterval(loadHistory,10000);loadHistory()}else{clearInterval(autoRefreshTimer);autoRefreshTimer=null}}
var servers=[];
function loadServersDropdown(){fetch('/api/servers').then(r=>r.json()).then(d=>{
    servers=d.servers||[];var h='<option value="">選擇服務器 Select Server</option>';
    servers.forEach(function(s){h+='<option value="'+s.id+'">'+escapeHtml(s.name)+'</option>'});
    document.getElementById('poolTestServerSelect').innerHTML=h;loadPool()
})}
loadServersDropdown();loadHistory();
</script>
`

const LocalModelsPage = `
<div class="card">
    <h2>本地模型映射 Local Model Mapping</h2>
    <button onclick="showModal('addMapModal')">新增映射 Add Mapping</button>
</div>
<div class="card">
    <table><thead><tr>
        <th>本地模型 Local Model</th><th>映射到服務器模型 Server Model</th><th>操作 Actions</th>
    </tr></thead>
    <tbody id="mapsTable"></tbody></table>
</div>
<div class="modal" id="addMapModal">
    <div class="modal-content">
        <div class="modal-header"><h3>新增映射 Add Mapping</h3><button class="close" onclick="hideModal('addMapModal')">&times;</button></div>
        <div class="form-group"><label>選擇服務器模型 Select Server Model</label><select id="mapServerModelID"><option value="">請選擇 Select</option></select></div>
        <div class="form-group"><label>本地模型名稱 Local Model Name</label><input type="text" id="localModel" placeholder="e.g. gpt-4"></div>
        <button onclick="addMapping()">新增 Add</button>
        <button onclick="hideModal('addMapModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<div class="modal" id="editMapModal">
    <div class="modal-content">
        <div class="modal-header"><h3>編輯映射 Edit Mapping</h3><button class="close" onclick="hideModal('editMapModal')">&times;</button></div>
        <input type="hidden" id="editMapId">
        <div class="form-group"><label>選擇服務器模型 Select Server Model</label><select id="editMapServerModelID"><option value="">請選擇 Select</option></select></div>
        <div class="form-group"><label>本地模型名稱 Local Model Name</label><input type="text" id="editLocalModel"></div>
        <button onclick="updateMapping()">儲存 Save</button>
        <button onclick="hideModal('editMapModal')" style="background:#95a5a6;margin-left:8px">取消 Cancel</button>
    </div>
</div>
<script>
var allModels=[];
function loadModels(){fetch('/api/server-models').then(r=>r.json()).then(d=>{
    allModels=d.server_models||[];
    var h='<option value="">請選擇 Select</option>';
    allModels.forEach(m=>{h+='<option value="'+m.id+'">'+escapeHtml(m.model_name)+'</option>'});
    document.getElementById('mapServerModelID').innerHTML=h;
    document.getElementById('editMapServerModelID').innerHTML=h;
    loadMaps()
})}
function loadMaps(){fetch('/api/local-model-maps').then(r=>r.json()).then(d=>{
    document.getElementById('mapsTable').innerHTML=(d.local_model_maps||[]).map(m=>{
        var sm=allModels.find(x=>x.id==m.server_model_id);
        return '<tr><td>'+escapeHtml(m.local_model)+'</td><td>'+(sm?escapeHtml(sm.model_name):m.server_model_id)+'</td><td class=actions><button class=small onclick="editMap(\''+m.id+'\')">編輯 Edit</button><button class="small danger" onclick="deleteMap(\''+m.id+'\')">刪除 Delete</button></td></tr>'
    }).join('')
})}
function addMapping(){fetch('/api/local-model-maps',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({server_model_id:document.getElementById('mapServerModelID').value,local_model:document.getElementById('localModel').value})}).then(r=>{if(r.ok){hideModal('addMapModal');document.getElementById('localModel').value='';loadMaps()}else{r.json().then(d=>alert(d.error))}})}
function editMap(id){fetch('/api/local-model-maps').then(r=>r.json()).then(d=>{
    var m=(d.local_model_maps||[]).find(x=>x.id==id);if(!m)return;
    document.getElementById('editMapId').value=m.id;document.getElementById('editMapServerModelID').value=m.server_model_id;document.getElementById('editLocalModel').value=m.local_model;showModal('editMapModal')
})}
function updateMapping(){fetch('/api/local-model-maps',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:document.getElementById('editMapId').value,server_model_id:document.getElementById('editMapServerModelID').value,local_model:document.getElementById('editLocalModel').value})}).then(r=>{if(r.ok){hideModal('editMapModal');loadMaps()}else{r.json().then(d=>alert(d.error))}})}
function deleteMap(id){if(confirm('確定要刪除? Confirm delete?'))fetch('/api/local-model-maps/'+id,{method:'DELETE'}).then(r=>{if(r.ok)loadMaps();else r.json().then(d=>alert(d.error))})}
loadModels();
</script>
`

const SettingsPage = `
<div class="card">
    <h2>系統設置 Settings</h2>
    <div class="form-group"><label>Timeout 分鐘 (3-15)</label><input type="number" id="timeout" min="3" max="15">
        <p class="help-text">API 代理請求的整體超時時間（包含連接 + 響應）。若服務器在時限內未回覆，請求將被中斷並觸發重試</p></div>
    <hr>
    <h3 style="font-size:16px;margin-bottom:15px;color:#2c3e50">進階設置 Advanced</h3>
    <div class="form-group checkbox"><label><input type="checkbox" id="enableNegativeWeight" style="width:auto"> 啟用負權重模式 Enable Negative Weight</label></div>
    <div class="form-group"><label>權重重置週期 Weight Reset Hours (2-8)</label><input type="number" id="weightResetHours" min="2" max="8"></div>
    <div class="form-row">
        <div class="form-group"><label>4xx 權重 Weight</label><input type="number" id="weight4xx" min="1"></div>
        <div class="form-group"><label>5xx 權重 Weight</label><input type="number" id="weight5xx" min="1"></div>
    </div>
    <div class="form-group"><label>最大重試次數 Max Retries</label><input type="number" id="maxRetries" min="0" max="10"></div>
    <hr>
    <h3 style="font-size:16px;margin-bottom:15px;color:#2c3e50">超時設置 Timeout</h3>
    <div class="form-group"><label>連接超時 Connect Timeout 秒</label><input type="number" id="connectTimeout" min="1" max="60"></div>
    <div class="form-row">
        <div class="form-group"><label>超時權重 Timeout Weight</label><input type="number" id="timeoutWeight" min="1"></div>
        <div class="form-group"><label>連接超時權重 Connect Timeout Weight</label><input type="number" id="connectTimeoutWeight" min="1"></div>
    </div>
    <div class="form-group checkbox"><label><input type="checkbox" id="enableRetryOnTimeout" style="width:auto"> 超時時重試 Retry on Timeout</label></div>
    <hr>
    <h3 style="font-size:16px;margin-bottom:15px;color:#2c3e50">健康檢查 Health Check</h3>
    <div class="form-group checkbox"><label><input type="checkbox" id="enableAutoCheckAPIKey" style="width:auto"> 自動檢查 API Key Auto Check</label></div>
    <div class="form-group"><label>檢查間隔 Check Interval (1~12 小時)</label><input type="number" id="autoCheckIntervalHours" min="1" max="12" value="6"></div>
    <hr>
    <div class="toolbar">
        <button onclick="saveSettings()" class="success">儲存設置 Save</button>
        <button onclick="resetWeights()" class="warning">重置所有權重 Reset Weights</button>
        <button onclick="reloadConfig()">重新載入配置 Reload</button>
    </div>
    <div id="settingsStatus" class="status-bar" style="display:none"></div>
</div>
<script>
function loadSettings(){fetch('/api/settings').then(r=>r.json()).then(d=>{
    document.getElementById('timeout').value=d.timeout||5;
    document.getElementById('enableNegativeWeight').checked=d.enable_negative_weight;
    document.getElementById('weightResetHours').value=d.weight_reset_hours||4;
    document.getElementById('weight4xx').value=d.weight_4xx||5;
    document.getElementById('weight5xx').value=d.weight_5xx||10;
    document.getElementById('maxRetries').value=d.max_retries||3;
    document.getElementById('connectTimeout').value=d.connect_timeout||30;
    document.getElementById('timeoutWeight').value=d.timeout_weight||8;
    document.getElementById('connectTimeoutWeight').value=d.connect_timeout_weight||12;
    document.getElementById('enableRetryOnTimeout').checked=d.enable_retry_on_timeout;
    document.getElementById('enableAutoCheckAPIKey').checked=d.enable_auto_check_api_key!==false;
    document.getElementById('autoCheckIntervalHours').value=d.auto_check_interval_hours||6;
})}
function saveSettings(){var s=JSON.stringify({timeout:parseInt(document.getElementById('timeout').value)||5,enable_negative_weight:document.getElementById('enableNegativeWeight').checked,weight_reset_hours:parseInt(document.getElementById('weightResetHours').value)||4,weight_4xx:parseInt(document.getElementById('weight4xx').value)||5,weight_5xx:parseInt(document.getElementById('weight5xx').value)||10,max_retries:parseInt(document.getElementById('maxRetries').value)||3,connect_timeout:parseInt(document.getElementById('connectTimeout').value)||30,timeout_weight:parseInt(document.getElementById('timeoutWeight').value)||8,connect_timeout_weight:parseInt(document.getElementById('connectTimeoutWeight').value)||12,enable_retry_on_timeout:document.getElementById('enableRetryOnTimeout').checked,enable_auto_check_api_key:document.getElementById('enableAutoCheckAPIKey').checked,auto_check_interval_hours:parseInt(document.getElementById('autoCheckIntervalHours').value)||6});
    fetch('/api/settings',{method:'POST',headers:{'Content-Type':'application/json'},body:s}).then(r=>{if(r.ok){showStatus('success','設置已儲存 Settings saved')}else{r.json().then(d=>showStatus('error',d.error))}})
}
function resetWeights(){if(confirm('確定重置所有權重? Reset all weights?'))fetch('/api/reset-weights',{method:'POST'}).then(r=>{if(r.ok)showStatus('success','權重已重置 Weights reset')})}
function reloadConfig(){fetch('/api/reload',{method:'POST'}).then(r=>{if(r.ok)showStatus('success','配置已重新載入 Config reloaded')})}
function showStatus(typ,msg){var el=document.getElementById('settingsStatus');el.style.display='block';el.className='status-bar '+typ;el.textContent=msg;setTimeout(()=>{el.style.display='none'},5000)}
loadSettings();
</script>
`

const TestResultPage = `
<div class="card">
    <h2>測試結果 Test Results</h2>
    <div class="toolbar">
        <button onclick="loadResults()">重新整理 Refresh</button>
        <label style="display:flex;align-items:center;gap:5px;font-size:13px">
            <input type="checkbox" id="autoRefresh" style="width:auto" onchange="toggleAutoRefresh()"> 自動刷新 Auto Refresh
        </label>
    </div>
</div>
<div class="card">
    <table><thead><tr>
        <th>API Key</th><th>狀態 Status</th><th>LED</th><th>耗時 Duration</th><th>錯誤 Error</th>
    </tr></thead>
    <tbody id="resultsTable"></tbody></table>
</div>
<script>
var autoRefreshTimer=null;
function loadResults(){fetch('/api/test-results').then(r=>r.json()).then(d=>{
    document.getElementById('resultsTable').innerHTML=(d.results||[]).map(r=>{
        return '<tr><td>'+escapeHtml(r.key_masked||'')+'</td><td>'+escapeHtml(r.status)+'</td><td>'+ledHtml(r.status)+'</td><td>'+escapeHtml(r.duration||'')+'</td><td>'+escapeHtml(r.error||'')+'</td></tr>'
    }).join('')
})}
function toggleAutoRefresh(){if(document.getElementById('autoRefresh').checked){autoRefreshTimer=setInterval(loadResults,10000);loadResults()}else{clearInterval(autoRefreshTimer);autoRefreshTimer=null}}
loadResults();
</script>
`

var Templates = template.Must(template.New("webui").Parse(PageTemplate + IndexPage + ServersPage + ServerModelsPage + APIKeysPage + PendingPoolPage + LocalModelsPage + SettingsPage + TestResultPage))
