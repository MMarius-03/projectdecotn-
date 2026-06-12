package main

// indexHTML este interfata, servita pe 127.0.0.1.
const indexHTML = `<!DOCTYPE html>
<html lang="ro">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Verificare diferente deconturi</title>
<style>
  :root { --bg:#0f172a; --card:#1e293b; --accent:#3b82f6; --ok:#22c55e; --warn:#f59e0b; --err:#ef4444; --muted:#94a3b8; }
  * { box-sizing: border-box; }
  body { margin:0; font-family: Segoe UI, system-ui, sans-serif; background:var(--bg); color:#e2e8f0; }
  .wrap { max-width: 760px; margin: 0 auto; padding: 28px 20px 60px; }
  h1 { font-size: 22px; margin: 0 0 4px; }
  .sub { color: var(--muted); font-size: 13px; margin-bottom: 22px; }
  .card { background: var(--card); border-radius: 14px; padding: 20px; margin-bottom: 18px; box-shadow: 0 8px 24px rgba(0,0,0,.25); }
  label { display:block; font-size:12px; text-transform:uppercase; letter-spacing:.04em; color:var(--muted); margin-bottom:8px; }
  select, input[type=text] { width:100%; padding:11px 12px; border-radius:9px; border:1px solid #334155; background:#0b1220; color:#e2e8f0; font-size:15px; }
  .row { display:flex; gap:12px; flex-wrap:wrap; align-items:flex-end; }
  .row > div { flex:1; min-width: 200px; }
  button { cursor:pointer; border:none; border-radius:9px; padding:12px 18px; font-size:15px; font-weight:600; color:#fff; background:var(--accent); transition:.15s; }
  button:hover { filter:brightness(1.1); }
  button:disabled { opacity:.5; cursor:not-allowed; }
  button.secondary { background:#475569; }
  button.success { background:var(--ok); }
  .btns { display:flex; gap:10px; flex-wrap:wrap; margin-top:16px; }
  .bar { height:10px; background:#0b1220; border-radius:6px; overflow:hidden; margin-top:14px; }
  .bar > i { display:block; height:100%; width:0; background:linear-gradient(90deg,#3b82f6,#22c55e); transition:width .2s; }
  .status { margin-top:12px; font-size:14px; min-height:20px; }
  .pill { display:inline-block; padding:3px 10px; border-radius:999px; font-size:12px; font-weight:600; margin-right:6px; }
  .pill.ok{background:rgba(34,197,94,.15);color:#4ade80;} .pill.warn{background:rgba(245,158,11,.15);color:#fbbf24;}
  .pill.err{background:rgba(239,68,68,.15);color:#f87171;} .pill.info{background:rgba(59,130,246,.15);color:#60a5fa;}
  .muted{color:var(--muted);font-size:13px;}
  .hidden{display:none;}
  .pathbox{font-family:Consolas,monospace;font-size:12px;color:var(--muted);word-break:break-all;margin-top:6px;}
  .summary{display:flex;gap:18px;flex-wrap:wrap;margin-top:12px;}
  .summary div{text-align:center;} .summary b{display:block;font-size:24px;}
  .incomplete{margin-top:14px;font-size:13px;color:#fbbf24;background:rgba(245,158,11,.08);border-radius:8px;padding:10px 14px;}
  .incomplete ul{margin:6px 0 0;padding-left:20px;color:#cbd5e1;} .incomplete li{margin:2px 0;}
</style>
</head>
<body>
<div class="wrap">
  <h1>Verificare diferente deconturi</h1>
  <div class="sub">Cauta randuri cu <b>DIF ≠ 0</b> sau <b>RETUR ≠ 0</b> in notele de cantar.</div>

  <div class="card">
    <div class="row">
      <div>
        <label>Selectare luna</label>
        <select id="month"></select>
      </div>
      <div style="flex:0 0 auto;min-width:auto;">
        <button id="btnVerify">Verificare</button>
      </div>
    </div>
    <div class="pathbox" id="pathbox"></div>
    <div class="btns">
      <button class="secondary" id="btnPath">Modificare cale</button>
      <button class="success hidden" id="btnOpen">Deschidere fisier generat</button>
      <button class="secondary hidden" id="btnFolder">Deschidere folder rapoarte</button>
    </div>
  </div>

  <div class="card" id="changePath" style="display:none;">
    <label>Cale noua catre folderul anului</label>
    <input type="text" id="newPath" placeholder="\\server\...\2026">
    <div class="btns">
      <button id="btnSavePath">Salveaza</button>
      <button class="secondary" id="btnCancelPath">Renunta</button>
    </div>
  </div>

  <div class="card">
    <div class="bar"><i id="bar"></i></div>
    <div class="status" id="status">Pregatit.</div>
    <div class="summary hidden" id="summary"></div>
    <div class="incomplete hidden" id="incomplete"></div>
  </div>
</div>

<script>
const $ = s => document.querySelector(s);
let lastReport = "", lastFolder = "";

async function loadState(){
  const r = await fetch('/api/state'); const s = await r.json();
  $('#pathbox').textContent = s.base_path + (s.error ? '  ['+s.error+']' : '');
  const sel = $('#month'); sel.innerHTML='';
  if(!s.months || !s.months.length){
    sel.innerHTML='<option>(nicio luna gasita)</option>';
    $('#btnVerify').disabled = true;
  } else {
    s.months.forEach(m=>{ const o=document.createElement('option'); o.value=m; o.textContent=m; sel.appendChild(o); });
    sel.selectedIndex = s.months.length-1;
    $('#btnVerify').disabled = false;
  }
  $('#newPath').value = s.base_path;
}

$('#btnPath').onclick = ()=>{ const c=$('#changePath'); c.style.display = c.style.display==='none'?'block':'none'; };
$('#btnCancelPath').onclick = ()=>{ $('#changePath').style.display='none'; };
$('#btnSavePath').onclick = async ()=>{
  const p = $('#newPath').value.trim(); if(!p) return;
  $('#btnSavePath').disabled=true;
  const r = await fetch('/api/setpath',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({base_path:p})});
  $('#btnSavePath').disabled=false;
  $('#changePath').style.display='none';
  await loadState();
};

$('#btnOpen').onclick = ()=>{ if(lastReport) fetch('/api/open?path='+encodeURIComponent(lastReport)); };
$('#btnFolder').onclick = ()=>{ if(lastFolder) fetch('/api/open?path='+encodeURIComponent(lastFolder)); };

$('#btnVerify').onclick = ()=>{
  const m = $('#month').value;
  $('#btnVerify').disabled=true; $('#btnOpen').classList.add('hidden'); $('#btnFolder').classList.add('hidden');
  $('#summary').classList.add('hidden'); $('#bar').style.width='0%';
  $('#status').innerHTML='<span class="pill info">Pornit</span> Se cauta fisierele...';
  lastReport=""; lastFolder="";

  const es = new EventSource('/api/verify?month='+encodeURIComponent(m));
  es.addEventListener('warn', e=>{
    $('#status').innerHTML = '<span class="pill warn">Avertisment</span> '+e.data;
  });
  es.addEventListener('progress', e=>{
    const d = JSON.parse(e.data);
    const pct = d.total? Math.round(d.done/d.total*100):0;
    $('#bar').style.width = pct+'%';
    $('#status').innerHTML = '<span class="pill info">'+pct+'%</span> Verificat '+d.done+' / '+d.total+' fisiere · '+d.with_dif+' cu diferente';
  });
  es.addEventListener('done', e=>{
    es.close(); $('#btnVerify').disabled=false; $('#bar').style.width='100%';
    const d = JSON.parse(e.data);
    let html='';
    html += '<span class="pill ok">Gata</span> ';
    if(d.error){ $('#status').innerHTML='<span class="pill err">Eroare</span> '+d.error; return; }
    if(d.rows>0){
      html += d.rows+' diferente in '+d.files_with_dif+' fisiere. Raport generat.';
    } else {
      html += 'Nu au fost gasite diferente.';
    }
    if(d.incomplete>0){ html += ' ('+d.incomplete+' note incomplete, vezi mai jos)'; }
    if(d.report){
      lastReport=d.report; lastFolder=d.folder;
      $('#btnOpen').classList.remove('hidden'); $('#btnFolder').classList.remove('hidden');
    }
    $('#status').innerHTML = html;
    $('#summary').classList.remove('hidden');
    $('#summary').innerHTML =
      '<div><b>'+d.checked+'</b><span class="muted">verificate</span></div>'+
      '<div><b style="color:#f87171">'+d.rows+'</b><span class="muted">diferente</span></div>'+
      '<div><b style="color:#fbbf24">'+d.incomplete+'</b><span class="muted">incomplete</span></div>'+
      '<div><b>'+d.ignored+'</b><span class="muted">ignorate</span></div>'+
      '<div><b>'+d.unprocessed+'</b><span class="muted">neprocesate</span></div>'+
      '<div><b>'+d.errors+'</b><span class="muted">erori</span></div>';

    const inc = $('#incomplete');
    if (d.incomplete_files && d.incomplete_files.length) {
      inc.classList.remove('hidden');
      inc.innerHTML = '<b>Note incomplete (CANT LIVRAT necompletat, excluse din raport):</b><ul>'+
        d.incomplete_files.map(f=>'<li>'+f+'</li>').join('')+'</ul>';
    } else {
      inc.classList.add('hidden'); inc.innerHTML='';
    }
  });
  es.onerror = ()=>{ es.close(); $('#btnVerify').disabled=false; $('#status').innerHTML='<span class="pill err">Eroare</span> Conexiune intrerupta.'; };
};

loadState();
</script>
</body>
</html>`
