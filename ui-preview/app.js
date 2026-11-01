const API = 'http://127.0.0.1:19346/api';
let downloads = [], filter = 'all', busy = false;
const $ = s => document.querySelector(s);
function toast(message){const el=$('#toast');el.textContent=message;el.classList.add('show');setTimeout(()=>el.classList.remove('show'),2400)}
function bytes(n){if(!n)return '—';const u=['B','KB','MB','GB','TB'];let i=0,v=n;while(v>=1024&&i<u.length-1){v/=1024;i++}return `${v.toFixed(i?1:0)} ${u[i]}`}
function state(x){const s=(x.Status||'').toLowerCase();if(s==='done'||s==='completed')return 'Completed';if(s.includes('error'))return 'Failed';if(s==='downloading'||s==='pausing...')return 'Downloading';if(s==='queued')return 'Queued';return 'Paused'}
async function request(path,options={}){const r=await fetch(API+path,{...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});if(!r.ok)throw new Error(await r.text());return r.json()}
function render(){const q=$('#search').value.toLowerCase();const rows=downloads.filter(x=>{const s=state(x);return(filter==='all'||filter==='active'&&s==='Downloading'||filter==='queued'&&s==='Queued'||filter==='done'&&s==='Completed'||filter==='failed'&&s==='Failed')&&(x.Name||'').toLowerCase().includes(q)});$('#rows').innerHTML=rows.map(x=>{const s=state(x),pct=Math.max(0,Math.min(100,x.Percent||0));const segmentBars=(x.Segments||[]).map((seg,i)=>`<i title="Segment ${i+1}: ${(seg.Pct||0).toFixed(1)}%"><b style="width:${Math.max(0,Math.min(100,seg.Pct||0))}%"></b></i>`).join('');return `<div class="download-row"><div class="file"><div class="fileicon">${x.Kind==='video'?'▶':x.Kind==='torrent'?'⇩':'▤'}</div><div class="filename">${escapeHTML(x.Name||x.URL)}<span class="sub">${bytes(x.Done)} / ${bytes(x.Total)} · ${x.Speed>0?bytes(x.Speed)+'/s':'—'}</span></div></div><div class="size">${bytes(x.Total)}</div><div><div class="progressline"><b style="width:${pct}%"></b></div><span class="pct">${pct.toFixed(1)}%</span>${segmentBars?`<div class="segment-mini" aria-label="${x.Segments.length} active segments">${segmentBars}</div><span class="pct">${x.Segments.length} transfer segments</span>`:''}</div><div class="status ${s==='Completed'?'done':s==='Queued'?'queued':''}">${s}</div><div class="actions">${s==='Downloading'?`<button data-act="pause" data-id="${x.Index}">Ⅱ</button>`:`<button data-act="start" data-id="${x.Index}" ${s==='Completed'?'disabled':''}>▶</button>`}<button data-act="remove" data-id="${x.Index}">×</button></div></div>`}).join('')||'<div class="download-row">No downloads match these filters.</div>';const count=s=>downloads.filter(x=>state(x)===s).length;$('#allCount').textContent=downloads.length;$('#activeCount').textContent=count('Downloading');$('#queuedCount').textContent=count('Queued');$('#doneCount').textContent=count('Completed');$('#statActive').textContent=count('Downloading');$('#statQueued').textContent=count('Queued');$('#statDone').textContent=count('Completed');}
function escapeHTML(s){return String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
async function refresh(){try{downloads=await request('/downloads');render();$('#engineStatus').textContent='Connected to Sabo DM';}catch(e){$('#engineStatus').textContent='App engine offline';}}
async function action(action,index=-1,url=''){if(busy)return;busy=true;try{await request('/action',{method:'POST',body:JSON.stringify({action,index,url})});await refresh();toast(action==='add'?'Download added to Sabo DM': 'Command sent to Sabo DM')}catch(e){toast(e.message||'Could not contact Sabo DM')}finally{busy=false}}
$('#add').onclick=()=>{const urls=$('#url').value.split(/\s+/).filter(Boolean);if(!urls.length)return toast('Paste at least one URL');(async()=>{for(const url of urls)await action('add',-1,url);$('#url').value=''})()};$('#url').addEventListener('keydown',e=>{if(e.key==='Enter')$('#add').click()});$('#search').oninput=render;
document.querySelectorAll('.nav[data-filter]').forEach(b=>b.onclick=()=>{filter=b.dataset.filter;document.querySelectorAll('.nav[data-filter]').forEach(x=>x.classList.toggle('active',x===b));render()});$('#rows').onclick=e=>{const b=e.target.closest('button[data-act]');if(b)action(b.dataset.act,Number(b.dataset.id))};$('#clear').onclick=()=>{$('#search').value='';filter='all';document.querySelectorAll('.nav[data-filter]').forEach(x=>x.classList.toggle('active',x.dataset.filter==='all'));render()};$('#pauseAll').onclick=async()=>{for(const x of downloads.filter(x=>state(x)==='Downloading'))await action('pause',x.Index)};$('#theme').onclick=()=>{document.body.classList.toggle('light');try{localStorage.setItem('sabo-theme',document.body.classList.contains('light')?'light':'dark')}catch(_){}};$('#addQueue').onclick=()=>toast('Queue creation is not yet connected');
refresh();setInterval(refresh,1200);


// Native WebView2 window controls. These call Go bindings, not browser-only UI tricks.
$('#winMin')?.addEventListener('click',()=>window.saboMinimize?.());
$('#winMax')?.addEventListener('click',()=>window.saboToggleMaximize?.());
$('#winClose')?.addEventListener('click',()=>{
  if(confirm('Exit Sabo DM? Active downloads may be interrupted.')) window.saboClose?.();
});

// Quick settings panel with a persistent appearance preference.
const settingsPanel=$('#settingsPanel');
function closeSettings(){settingsPanel?.classList.add('hidden')}
$('#settings')?.addEventListener('click',()=>{
  const light=document.body.classList.contains('light');
  $('#appearance').value=light?'light':'dark';
  settingsPanel?.classList.remove('hidden');
});
$('#settingsClose')?.addEventListener('click',closeSettings);
$('#settingsDone')?.addEventListener('click',()=>{
  document.body.classList.toggle('light',$('#appearance').value==='light');
  try{localStorage.setItem('sabo-theme',$('#appearance').value)}catch(_){}
  closeSettings();
});
settingsPanel?.addEventListener('click',e=>{if(e.target===settingsPanel)closeSettings()});
try{if(localStorage.getItem('sabo-theme')==='light')document.body.classList.add('light')}catch(_){}

// Apply the global speed limit to the Go transfer engine. Values are KiB/s; zero means unlimited.
const speedSelect=document.querySelector('.detailfoot select');
if(speedSelect){
  speedSelect.id='speedLimit';
  speedSelect.addEventListener('change',async()=>{
    const map={'Unlimited':0,'1 MB/s':1024,'5 MB/s':5120,'10 MB/s':10240};
    const kb=map[speedSelect.value];
    if(kb===undefined)return toast('Unsupported speed preset');
    try{await request('/action',{method:'POST',body:JSON.stringify({action:'set-speed',index:kb})});toast('Global speed limit updated')}
    catch(e){toast(e.message||'Could not update speed limit')}
  });
}
