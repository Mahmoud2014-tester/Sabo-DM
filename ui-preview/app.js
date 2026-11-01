'use strict';

const API = 'http://127.0.0.1:19346/api';
const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => Array.from(document.querySelectorAll(selector));

let downloads = [];
let seenDownloadURLs = null;
let noticeTimer = null;
let meta = { queues: ['Default'], categories: ['Files', 'Videos', 'Music', 'Documents', 'Torrents'], version: '—' };
let filter = 'all';
let queueFilter = 'all';
let categoryFilter = 'all';
let sortMode = 'date';
let selectedIndex = null;
let requestBusy = false;
let lastError = '';
let updateInfo = null;
let toolPoll = null;

const I18N = {
  ar: {
    "Downloads":"التنزيلات","Manage your files, media and torrents":"إدارة ملفاتك ووسائطك والتورنت",
    "LIBRARY":"المكتبة","All downloads":"كل التنزيلات","Active":"النشطة","Queued":"قيد الانتظار","Completed":"مكتملة","Failed":"فاشلة",
    "QUEUES":"قوائم الانتظار","CATEGORIES":"التصنيفات","All categories":"كل التصنيفات","Music":"الموسيقى","Videos":"الفيديوهات","Documents":"المستندات","Torrents":"التورنت",
    "ACTIVE DOWNLOADS":"التنزيلات النشطة","Currently transferring":"جارٍ النقل","Waiting for a free slot":"في انتظار دورها","Successfully downloaded":"تم تنزيلها بنجاح","DOWNLOAD SPEED":"سرعة التنزيل","Current combined throughput":"السرعة الإجمالية الحالية",
    "Add new download":"إضافة تنزيل جديد","Paste a URL, magnet link, or batch of links":"أدخل رابطًا أو رابط Magnet أو عدة روابط","＋ NEW TASK":"＋ مهمة جديدة","＋ Add download":"＋ إضافة تنزيل",
    "Media output":"صيغة الوسائط","Video (best available)":"فيديو (أفضل جودة متاحة)","Audio · MP3":"صوت · MP3","Audio · M4A":"صوت · M4A","Audio · AAC":"صوت · AAC","Audio · Opus":"صوت · Opus","Audio · WAV":"صوت · WAV","Audio · FLAC":"صوت · FLAC","Audio · Original format":"صوت · الصيغة الأصلية",
    "Audio output selection applies to supported video/media page URLs. Conversion requires FFmpeg.":"اختيار صيغة الصوت يعمل مع صفحات الفيديو والوسائط المدعومة. التحويل يتطلب FFmpeg.",
    "↳ Supports HTTP / HTTPS / magnet links":"↳ يدعم روابط HTTP وHTTPS وMagnet","Save to:":"الحفظ في:",
    "Date added ↓":"تاريخ الإضافة ↓","Name A–Z":"الاسم أ–ي","Clear filters":"مسح عوامل التصفية","Ⅱ Pause all":"Ⅱ إيقاف الكل مؤقتًا","Clear finished":"مسح المكتمل",
    "FILE NAME":"اسم الملف","SIZE":"الحجم","PROGRESS":"التقدم","STATUS":"الحالة","ACTIONS":"الإجراءات",
    "Download details":"تفاصيل التنزيل","Select a download to see its live transfer details":"اختر تنزيلًا لعرض تفاصيل النقل المباشرة","ENGINE SYNC":"مزامنة المحرك","Transfers":"عمليات النقل","Progress":"التقدم","Engine":"المحرك","Connected":"متصل","Parallel downloads":"التنزيلات المتوازية","Speed limit":"حد السرعة","Unlimited":"بلا حدود",
    "Sabo DM Settings":"إعدادات Sabo DM","Application tools, updates and appearance":"أدوات التطبيق والتحديثات والمظهر","Appearance":"المظهر","Theme":"السمة","Dark":"داكن","Light":"فاتح","Language":"اللغة","English (Primary)":"الإنجليزية (أساسية)","العربية":"العربية","Français":"الفرنسية",
    "Media tools":"أدوات الوسائط","Checking… — required for MP3, AAC, Opus, WAV and FLAC conversion.":"جارٍ التحقق… مطلوبة للتحويل إلى MP3 وAAC وOpus وWAV وFLAC.","Install FFmpeg":"تثبيت FFmpeg","yt-dlp":"yt-dlp","Install / Update":"تثبيت / تحديث",
    "Software update":"تحديث البرنامج","Update Version":"تحديث الإصدار","Check GitHub for the latest stable release.":"تحقق من GitHub لمعرفة أحدث إصدار مستقر.","Check for updates":"التحقق من وجود تحديثات","Download and install update":"تنزيل التحديث وتثبيته","View release notes on GitHub ↗":"عرض ملاحظات الإصدار على GitHub ↗","Done":"تم",
    "Local desktop engine":"محرك سطح المكتب المحلي","All download commands are handled by the Go engine":"يتولى محرك Go جميع أوامر التنزيل","Create download queue":"إنشاء قائمة تنزيل","Give this queue a name. You can assign downloads to it from the dashboard.":"أدخل اسم القائمة. يمكنك إسناد التنزيلات إليها من لوحة التحكم.","Queue name":"اسم القائمة","Cancel":"إلغاء","Create queue":"إنشاء قائمة",
    "You are running the latest release ({version}).":"أنت تستخدم أحدث إصدار ({version}).",
    "Update available: {latest} (current: {current})":"يتوفر تحديث: {latest} (الإصدار الحالي: {current})",
    "Checking the latest stable GitHub release…":"جارٍ التحقق من أحدث إصدار مستقر على GitHub…",
    "Downloading and verifying the update…":"جارٍ تنزيل التحديث والتحقق منه…",
    "Installing {version}. Sabo DM will restart automatically.":"جارٍ تثبيت {version}. سيُعاد تشغيل Sabo DM تلقائيًا.",
    "Update verified. Restarting Sabo DM…":"تم التحقق من التحديث. جارٍ إعادة تشغيل Sabo DM…"
  },
  fr: {
    "Downloads":"Téléchargements","Manage your files, media and torrents":"Gérez vos fichiers, médias et torrents","LIBRARY":"BIBLIOTHÈQUE","All downloads":"Tous les téléchargements","Active":"Actifs","Queued":"En attente","Completed":"Terminés","Failed":"Échoués",
    "QUEUES":"FILES D’ATTENTE","CATEGORIES":"CATÉGORIES","All categories":"Toutes les catégories","Music":"Musique","Videos":"Vidéos","Documents":"Documents","Torrents":"Torrents",
    "ACTIVE DOWNLOADS":"TÉLÉCHARGEMENTS ACTIFS","Currently transferring":"Transfert en cours","Waiting for a free slot":"En attente d’un emplacement","Successfully downloaded":"Téléchargés avec succès","DOWNLOAD SPEED":"VITESSE DE TÉLÉCHARGEMENT","Current combined throughput":"Débit total actuel",
    "Add new download":"Ajouter un téléchargement","Paste a URL, magnet link, or batch of links":"Collez une URL, un lien magnet ou plusieurs liens","＋ NEW TASK":"＋ NOUVELLE TÂCHE","＋ Add download":"＋ Ajouter",
    "Media output":"Format média","Video (best available)":"Vidéo (meilleure qualité)","Audio · MP3":"Audio · MP3","Audio · M4A":"Audio · M4A","Audio · AAC":"Audio · AAC","Audio · Opus":"Audio · Opus","Audio · WAV":"Audio · WAV","Audio · FLAC":"Audio · FLAC","Audio · Original format":"Audio · Format original",
    "Audio output selection applies to supported video/media page URLs. Conversion requires FFmpeg.":"Le format audio s’applique aux pages vidéo/média compatibles. La conversion nécessite FFmpeg.",
    "↳ Supports HTTP / HTTPS / magnet links":"↳ Liens HTTP / HTTPS / magnet acceptés","Save to:":"Enregistrer dans :",
    "Date added ↓":"Date d’ajout ↓","Name A–Z":"Nom A–Z","Clear filters":"Effacer les filtres","Ⅱ Pause all":"Ⅱ Tout suspendre","Clear finished":"Effacer les terminés",
    "FILE NAME":"NOM DU FICHIER","SIZE":"TAILLE","PROGRESS":"PROGRESSION","STATUS":"ÉTAT","ACTIONS":"ACTIONS",
    "Download details":"Détails du téléchargement","Select a download to see its live transfer details":"Sélectionnez un téléchargement pour voir son transfert","ENGINE SYNC":"SYNCHRONISATION","Transfers":"Transferts","Progress":"Progression","Engine":"Moteur","Connected":"Connecté","Parallel downloads":"Téléchargements parallèles","Speed limit":"Limite de vitesse","Unlimited":"Illimitée",
    "Sabo DM Settings":"Paramètres Sabo DM","Application tools, updates and appearance":"Outils, mises à jour et apparence","Appearance":"Apparence","Theme":"Thème","Dark":"Sombre","Light":"Clair","Language":"Langue","English (Primary)":"Anglais (principal)","العربية":"Arabe","Français":"Français",
    "Media tools":"Outils multimédias","Checking… — required for MP3, AAC, Opus, WAV and FLAC conversion.":"Vérification… requis pour convertir en MP3, AAC, Opus, WAV et FLAC.","Install FFmpeg":"Installer FFmpeg","yt-dlp":"yt-dlp","Install / Update":"Installer / Mettre à jour",
    "Software update":"Mise à jour du logiciel","Update Version":"Mettre à jour la version","Check GitHub for the latest stable release.":"Rechercher la dernière version stable sur GitHub.","Check for updates":"Rechercher des mises à jour","Download and install update":"Télécharger et installer","View release notes on GitHub ↗":"Voir les notes de version sur GitHub ↗","Done":"Terminé",
    "Local desktop engine":"Moteur local","All download commands are handled by the Go engine":"Toutes les commandes sont gérées par le moteur Go","Create download queue":"Créer une file d’attente","Give this queue a name. You can assign downloads to it from the dashboard.":"Nommez cette file. Vous pourrez y affecter des téléchargements.","Queue name":"Nom de la file","Cancel":"Annuler","Create queue":"Créer",
    "You are running the latest release ({version}).":"Vous utilisez la dernière version ({version}).",
    "Update available: {latest} (current: {current})":"Mise à jour disponible : {latest} (version actuelle : {current})",
    "Checking the latest stable GitHub release…":"Vérification de la dernière version stable sur GitHub…",
    "Downloading and verifying the update…":"Téléchargement et vérification de la mise à jour…",
    "Installing {version}. Sabo DM will restart automatically.":"Installation de {version}. Sabo DM redémarrera automatiquement.",
    "Update verified. Restarting Sabo DM…":"Mise à jour vérifiée. Redémarrage de Sabo DM…"
  }
};
function tr(key, values = {}) { const lang = document.documentElement.lang || 'en'; let result = (I18N[lang] || {})[key] || key; for (const [name,value] of Object.entries(values)) result = result.replaceAll(`{${name}}`, String(value)); return result; }
const originalTextNodes = new WeakMap();
function applyLanguage(language) {
  const lang = ['ar','fr'].includes(language) ? language : 'en';
  const dict = I18N[lang] || {};
  document.documentElement.lang = lang;
  document.documentElement.dir = lang === 'ar' ? 'rtl' : 'ltr';
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  let node;
  while ((node = walker.nextNode())) {
    if (!originalTextNodes.has(node)) originalTextNodes.set(node, node.nodeValue);
    const raw = originalTextNodes.get(node);
    const key = raw.trim();
    node.nodeValue = dict[key] ? raw.replace(key, dict[key]) : raw;
  }
  const attrs = {
    '#url': {placeholder: {'https://example.com/file.zip  (one URL per line)': {'ar':'https://example.com/file.zip (رابط واحد في كل سطر)','fr':'https://example.com/file.zip (une URL par ligne)'}}},
    '#search': {placeholder: {'⌕  Search downloads…': {'ar':'⌕  ابحث في التنزيلات…','fr':'⌕  Rechercher…'}}},
    '#queueName': {placeholder: {'For example: Work files': {'ar':'مثال: ملفات العمل','fr':'Ex. : fichiers professionnels'}}}
  };
  Object.entries(attrs).forEach(([selector, props]) => Object.entries(props).forEach(([attr, values]) => {
    const el = document.querySelector(selector); if (el && values[Object.keys(values)[0]][lang]) el.setAttribute(attr, values[Object.keys(values)[0]][lang]);
  }));
  try { localStorage.setItem('sabo-language', lang); } catch (_) {}
}

function toast(message, kind = 'ok') {
  const el = $('#toast');
  el.textContent = String(message || '');
  el.dataset.kind = kind;
  el.classList.add('show');
  clearTimeout(toast._timer);
  toast._timer = setTimeout(() => el.classList.remove('show'), 3200);
}

function showDownloadNotice(item) {
  const panel = $('#downloadNotice');
  if (!panel) return;
  $('#downloadNoticeName').textContent = String(item.name || 'Preparing download');
  $('#downloadNoticeText').textContent = (item.status === 'Queued' || item.status === 'queued') ? 'Added to download queue' : 'Download started';
  panel.classList.add('show');
  clearTimeout(noticeTimer);
  noticeTimer = setTimeout(() => panel.classList.remove('show'), 6500);
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}

function bytes(value) {
  const n = Number(value) || 0;
  if (n <= 0) return '—';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = n, i = 0;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(i ? 1 : 0)} ${units[i]}`;
}

function humanSpeed(value) { return `${bytes(value)}/s`; }

function downloadState(item) {
  const raw = String(item.status || '').toLowerCase();
  if (raw === 'done' || raw === 'completed') return 'Completed';
  if (raw.includes('error') || raw.includes('failed')) return 'Failed';
  if (raw === 'downloading' || raw === 'pausing...') return 'Downloading';
  if (raw === 'queued') return 'Queued';
  return 'Paused';
}

async function request(path, options = {}) {
  const response = await fetch(API + path, {
    cache: 'no-store',
    ...options,
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }
  });
  const bodyText = await response.text();
  let body = {};
  if (bodyText) {
    try { body = JSON.parse(bodyText); }
    catch { body = { message: bodyText }; }
  }
  if (!response.ok) throw new Error(body.error || body.message || bodyText || `Request failed (${response.status})`);
  return body;
}

async function postAction(action, extra = {}) {
  return request('/action', { method: 'POST', body: JSON.stringify({ action, ...extra }) });
}

function iconFor(item) {
  if (item.kind === 'video') return '▶';
  if (item.kind === 'torrent') return '⇩';
  const cat = String(item.category || '').toLowerCase();
  if (cat === 'music') return '♫';
  if (cat === 'documents') return '▤';
  return '▣';
}

function renderQueues() {
  const host = $('#queuesList');
  const names = Array.isArray(meta.queues) && meta.queues.length ? meta.queues : ['Default'];
  host.innerHTML = names.map(name => {
    const count = downloads.filter(item => (item.queue || 'Default') === name).length;
    return `<button class="nav queue-nav ${queueFilter === name ? 'active' : ''}" data-queue="${escapeHTML(name)}"><span class="queue-dot">●</span><span>${escapeHTML(name)}</span><i>${count}</i></button>`;
  }).join('');
  host.querySelectorAll('[data-queue]').forEach(button => button.addEventListener('click', () => {
    queueFilter = button.dataset.queue;
    renderQueues(); render();
  }));
}

function filteredDownloads() {
  const query = ($('#search')?.value || '').trim().toLowerCase();
  let rows = downloads.filter(item => {
    const status = downloadState(item);
    const matchesState = filter === 'all' ||
      (filter === 'active' && status === 'Downloading') ||
      (filter === 'queued' && status === 'Queued') ||
      (filter === 'done' && status === 'Completed') ||
      (filter === 'failed' && status === 'Failed');
    const matchesQueue = queueFilter === 'all' || (item.queue || 'Default') === queueFilter;
    const matchesCategory = categoryFilter === 'all' || (item.category || 'Files') === categoryFilter;
    const haystack = `${item.name || ''} ${item.url || ''}`.toLowerCase();
    return matchesState && matchesQueue && matchesCategory && haystack.includes(query);
  });
  rows.sort((a, b) => sortMode === 'name'
    ? String(a.name || a.url).localeCompare(String(b.name || b.url))
    : Number(b.index) - Number(a.index));
  return rows;
}

function renderRows() {
  const rows = filteredDownloads();
  $('#rows').innerHTML = rows.map(item => {
    const status = downloadState(item);
    const percent = Math.max(0, Math.min(100, Number(item.percent) || 0));
    const segments = Array.isArray(item.segments) ? item.segments : [];
    const bars = segments.map((segment, i) => {
      const p = Math.max(0, Math.min(100, Number(segment.pct) || 0));
      return `<i title="Segment ${i + 1}: ${p.toFixed(1)}%"><b style="width:${p}%"></b></i>`;
    }).join('');
    const safeIndex = Number.isInteger(item.index) ? item.index : -1;
    const disabled = safeIndex < 0;
    const primaryAction = status === 'Downloading'
      ? `<button data-act="pause" data-id="${safeIndex}" ${disabled ? 'disabled' : ''} title="Pause">Ⅱ</button>`
      : `<button data-act="start" data-id="${safeIndex}" ${disabled || status === 'Completed' ? 'disabled' : ''} title="Start or resume">▶</button>`;
    const canOpen = status === 'Completed';
    return `<div class="download-row ${selectedIndex === safeIndex ? 'selected' : ''}" data-row="${safeIndex}">
      <div class="file"><div class="fileicon">${iconFor(item)}</div><div class="filename" title="${escapeHTML(item.url)}">${escapeHTML(item.name || item.url || 'Preparing download')}<span class="sub">${bytes(item.done)} / ${bytes(item.total)} · ${Number(item.speed) > 0 ? humanSpeed(item.speed) : '—'} · ${escapeHTML(item.queue || 'Default')}</span></div></div>
      <div class="size">${bytes(item.total)}</div>
      <div><div class="progressline"><b style="width:${percent}%"></b></div><span class="pct">${percent.toFixed(1)}%</span>${bars ? `<div class="segment-mini">${bars}</div><span class="pct">${segments.length} segments</span>` : ''}</div>
      <div class="status ${status === 'Completed' ? 'done' : status === 'Queued' ? 'queued' : status === 'Failed' ? 'failed' : ''}">${status}</div>
      <div class="actions">${primaryAction}<button data-act="open-folder" data-id="${safeIndex}" ${disabled ? 'disabled' : ''} title="Open containing folder">▱</button><button data-act="open-file" data-id="${safeIndex}" ${disabled || !canOpen ? 'disabled' : ''} title="Open file">↗</button><button data-act="remove" data-id="${safeIndex}" ${disabled ? 'disabled' : ''} title="Remove from list">×</button></div>
    </div>`;
  }).join('') || '<div class="empty-state"><b>No downloads found</b><span>Try a different filter or add a URL above.</span></div>';
  let lang = 'en'; try { lang = localStorage.getItem('sabo-language') || 'en'; } catch (_) {}
  applyLanguage(lang);
}

function renderStats() {
  const count = status => downloads.filter(item => downloadState(item) === status).length;
  $('#allCount').textContent = downloads.length;
  $('#activeCount').textContent = count('Downloading');
  $('#queuedCount').textContent = count('Queued');
  $('#doneCount').textContent = count('Completed');
  $('#failedCount').textContent = count('Failed');
  $('#statActive').textContent = count('Downloading');
  $('#statQueued').textContent = count('Queued');
  $('#statDone').textContent = count('Completed');
  const speed = downloads.reduce((sum, item) => sum + (Number(item.speed) || 0), 0);
  $('#statSpeed').textContent = `${bytes(speed)}/s`;
}

function renderDetails() {
  const selected = downloads.find(item => item.index === selectedIndex) || downloads[0];
  if (!selected) {
    $('#detailText').textContent = 'Select a download to see its live transfer details';
    $('#detailTransfer').style.width = '0%';
    $('#detailProgress').style.width = '0%';
    $('#detailSegmentCount').textContent = '—';
    $('#detailPercent').textContent = '—';
    return;
  }
  const percent = Math.max(0, Math.min(100, Number(selected.percent) || 0));
  const segments = selected.segments || [];
  const activeSegments = segments.length;
  $('#detailText').textContent = `${selected.name || selected.url} · ${downloadState(selected)} · ${humanSpeed(selected.speed || 0)}`;
  $('#detailTransfer').style.width = `${activeSegments ? Math.min(100, activeSegments / 32 * 100) : 0}%`;
  $('#detailProgress').style.width = `${percent}%`;
  $('#detailSegmentCount').textContent = activeSegments ? `${activeSegments} active` : 'Single stream / not started';
  $('#detailPercent').textContent = `${percent.toFixed(1)}%`;
}

function render() { renderQueues(); renderRows(); renderStats(); renderDetails(); }

async function refresh() {
  try {
    const [items, info] = await Promise.all([request('/downloads'), request('/meta')]);
    const nextDownloads = Array.isArray(items) ? items : [];
    if (seenDownloadURLs !== null) {
      const fresh = nextDownloads.filter(item => item.url && !seenDownloadURLs.has(item.url));
      if (fresh.length) showDownloadNotice(fresh[0]);
    }
    seenDownloadURLs = new Set(nextDownloads.map(item => item.url).filter(Boolean));
    downloads = nextDownloads;
    meta = { ...meta, ...(info || {}) };
    $('#engineStatus').textContent = 'Connected to Sabo DM';
    $('#engineReady').textContent = 'Connected';
    $('#appVersion').textContent = meta.version || '—';
    $('#parallelCount').textContent = meta.downloadsAtOnce ?? '—';
    $('#footerStatus').textContent = '● Sabo DM engine connected';
    render();
  } catch (error) {
    $('#engineStatus').textContent = 'App engine offline';
    $('#engineReady').textContent = 'Offline';
    $('#footerStatus').textContent = '● Cannot connect to Go engine';
    lastError = error.message;
  }
}

async function runAction(action, extra = {}, successMessage = 'Action completed') {
  if (requestBusy) return;
  requestBusy = true;
  try {
    await postAction(action, extra);
    await refresh();
    toast(successMessage);
  } catch (error) {
    toast(error.message || 'Sabo DM could not complete that action', 'error');
  } finally { requestBusy = false; }
}

async function addURLs() {
  const raw = $('#url').value.trim();
  const urls = raw.split(/[\r\n\s]+/).filter(Boolean);
  if (!urls.length) { toast('Paste at least one HTTP, HTTPS, or magnet URL', 'error'); return; }
  const invalid = urls.find(url => !/^(https?:\/\/|magnet:\?)/i.test(url));
  if (invalid) { toast(`Unsupported link: ${invalid}`, 'error'); return; }
  $('#add').disabled = true;
  let added = 0;
  try {
    for (const url of urls) {
      const mode = $('#mediaMode').value || 'video';
      const audioOnly = mode !== 'video';
      await postAction('add', { url, queue: queueFilter === 'all' ? 'Default' : queueFilter, audioOnly, audioFormat: audioOnly ? mode : 'original' });
      added++;
    }
    $('#url').value = '';
    await refresh();
    toast(`${added} download${added === 1 ? '' : 's'} added to Sabo DM`);
  } catch (error) {
    toast(`${added} accepted. ${error.message}`, 'error');
    await refresh();
  } finally { $('#add').disabled = false; }
}

function setMainFilter(next) {
  filter = next;
  $$('.nav[data-filter]').forEach(button => button.classList.toggle('active', button.dataset.filter === next));
  render();
}

function resetFilters() {
  $('#search').value = '';
  filter = 'all'; queueFilter = 'all'; categoryFilter = 'all';
  $$('.nav[data-filter]').forEach(button => button.classList.toggle('active', button.dataset.filter === 'all'));
  $$('.nav[data-category]').forEach(button => button.classList.toggle('active', button.dataset.category === 'all'));
  render();
}

function showModal(id) { $(id).classList.remove('hidden'); }
function hideModal(id) { $(id).classList.add('hidden'); }

async function createQueue() {
  const name = $('#queueName').value.trim();
  if (!name) { toast('Enter a queue name', 'error'); return; }
  try {
    await postAction('add-queue', { name });
    await refresh();
    queueFilter = name;
    hideModal('#queuePanel');
    $('#queueName').value = '';
    render();
    toast(`Queue “${name}” created`);
  } catch (error) { toast(error.message, 'error'); }
}

async function deleteCurrentQueue() {
  if (queueFilter === 'all' || queueFilter === 'Default') { toast('Select a custom queue first', 'error'); return; }
  const name = queueFilter;
  if (!confirm(`Delete queue “${name}”? Its downloads will be moved to Default; files will not be deleted.`)) return;
  try {
    await postAction('delete-queue', { queue: name });
    queueFilter = 'all';
    await refresh();
    toast(`Queue “${name}” deleted`);
  } catch (error) { toast(error.message, 'error'); }
}

async function checkUpdate() {
  $('#checkUpdate').disabled = true;
  $('#updateStatus').textContent = tr('Checking the latest stable GitHub release…');
  $('#installUpdate').classList.add('hidden');
  $('#releaseLink').classList.add('hidden');
  try {
    updateInfo = await request('/update/check');
    $('#updateStatus').textContent = updateInfo.updateAvailable
      ? tr('Update available: {latest} (current: {current})', { latest: updateInfo.latestVersion, current: updateInfo.currentVersion })
      : tr('You are running the latest release ({version}).', { version: updateInfo.currentVersion });
    if (updateInfo.releaseUrl) { $('#releaseLink').href = updateInfo.releaseUrl; $('#releaseLink').classList.remove('hidden'); }
    if (updateInfo.updateAvailable && updateInfo.hasWindowsAsset) $('#installUpdate').classList.remove('hidden');
    else if (updateInfo.updateAvailable) $('#updateStatus').textContent += ' The release has no Windows executable asset.';
  } catch (error) { $('#updateStatus').textContent = error.message; }
  finally { $('#checkUpdate').disabled = false; }
}

async function installUpdate() {
  if (!updateInfo?.updateAvailable || !updateInfo?.hasWindowsAsset) return;
  if (!confirm(`Download and install Sabo DM ${updateInfo.latestVersion}? The application will close and restart.`)) return;
  $('#installUpdate').disabled = true;
  $('#updateStatus').textContent = tr('Downloading and verifying the update…');
  try {
    const result = await request('/update/install', { method: 'POST', body: '{}' });
    $('#updateStatus').textContent = tr('Installing {version}. Sabo DM will restart automatically.', { version: result.latestVersion });
    toast(tr('Update verified. Restarting Sabo DM…'));
  } catch (error) {
    $('#updateStatus').textContent = error.message;
    $('#installUpdate').disabled = false;
  }
}

async function refreshCapabilities() {
  try {
    const info = await request('/capabilities');
    $('#ffmpegStatus').textContent = info.ffmpegReady ? 'FFmpeg and FFprobe are installed and working.' : 'Required for MP3, M4A, AAC, Opus, WAV and FLAC conversion.';
    $('#installFFmpeg').textContent = info.ffmpegReady ? 'Reinstall / Repair' : 'Install FFmpeg';
    $('#ytdlpStatus').textContent = info.ytDlpReady ? 'yt-dlp is installed.' : 'Required for supported video and media sites.';
    $('#updateYtDlp').textContent = info.ytDlpReady ? 'Update yt-dlp' : 'Install yt-dlp';
  } catch (error) {
    $('#ffmpegStatus').textContent = error.message;
    $('#ytdlpStatus').textContent = 'Cannot check while the engine is offline.';
  }
}

async function startToolJob(name, path) {
  const button = $(name);
  button.disabled = true;
  try {
    await request(path, { method: 'POST', body: '{}' });
    toast('Tool operation started. You can keep using Sabo DM.');
    if (toolPoll) clearInterval(toolPoll);
    toolPoll = setInterval(async () => {
      try {
        const job = await request('/tools/status');
        if (job.state === 'running') {
          $('#ffmpegStatus').textContent = `${job.name}: working…`;
          $('#ytdlpStatus').textContent = `${job.name}: working…`;
          return;
        }
        clearInterval(toolPoll); toolPoll = null;
        if (job.state === 'failed') toast(job.error || `${job.name} failed`, 'error');
        else if (job.state === 'completed') toast(`${job.name} completed`);
        await refreshCapabilities();
        button.disabled = false;
      } catch (_) { /* retry on next polling tick */ }
    }, 1000);
  } catch (error) {
    button.disabled = false;
    toast(error.message, 'error');
  }
}

function bindEvents() {
  $('#downloadNoticeClose')?.addEventListener('click', () => $('#downloadNotice').classList.remove('show'));
  $('#add').addEventListener('click', addURLs);
  $('#url').addEventListener('keydown', event => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') addURLs(); });
  $('#search').addEventListener('input', renderRows);
  $$('.nav[data-filter]').forEach(button => button.addEventListener('click', () => setMainFilter(button.dataset.filter)));
  $$('.nav[data-category]').forEach(button => button.addEventListener('click', () => {
    categoryFilter = button.dataset.category;
    $$('.nav[data-category]').forEach(item => item.classList.toggle('active', item === button));
    renderRows();
  }));
  $('#rows').addEventListener('click', async event => {
    const button = event.target.closest('button[data-act]');
    if (!button || button.disabled) return;
    const index = Number(button.dataset.id);
    if (!Number.isInteger(index) || index < 0) { toast('This download row is out of date. Refreshing…', 'error'); return refresh(); }
    const action = button.dataset.act;
    await runAction(action, { index }, action === 'remove' ? 'Download removed from the list' : `${action} command sent`);
  });
  $('#rows').addEventListener('click', event => {
    if (event.target.closest('button')) return;
    const row = event.target.closest('[data-row]');
    if (!row) return;
    selectedIndex = Number(row.dataset.row);
    renderRows(); renderDetails();
  });
  $('#clear').addEventListener('click', resetFilters);
  $('#pauseAll').addEventListener('click', async () => {
    const active = downloads.filter(item => downloadState(item) === 'Downloading');
    for (const item of active) await runAction('pause', { index: item.index }, 'Download paused');
  });
  $('#clearFinished').addEventListener('click', () => runAction('clear-done', {}, 'Finished downloads cleared'));
  $('#sortDate').addEventListener('click', () => { sortMode = 'date'; $('#sortDate').classList.add('selected'); $('#sortName').classList.remove('selected'); renderRows(); });
  $('#sortName').addEventListener('click', () => { sortMode = 'name'; $('#sortName').classList.add('selected'); $('#sortDate').classList.remove('selected'); renderRows(); });
  $('#addQueue').addEventListener('click', () => showModal('#queuePanel'));
  $('#deleteQueue').addEventListener('click', deleteCurrentQueue);
  $('#queueCreate').addEventListener('click', createQueue);
  $('#queueName').addEventListener('keydown', event => { if (event.key === 'Enter') createQueue(); });
  $('#queueClose').addEventListener('click', () => hideModal('#queuePanel'));
  $('#queueCancel').addEventListener('click', () => hideModal('#queuePanel'));
  $('#theme').addEventListener('click', () => {
    document.body.classList.toggle('light');
    try { localStorage.setItem('sabo-theme', document.body.classList.contains('light') ? 'light' : 'dark'); } catch (_) {}
  });
  $('#settings').addEventListener('click', () => { showModal('#settingsPanel'); refreshCapabilities(); });
  $('#settingsClose').addEventListener('click', () => hideModal('#settingsPanel'));
  $('#settingsDone').addEventListener('click', () => {
    document.body.classList.toggle('light', $('#appearance').value === 'light');
    try { localStorage.setItem('sabo-theme', $('#appearance').value); } catch (_) {}
    hideModal('#settingsPanel');
  });
  $('#language').addEventListener('change', () => applyLanguage($('#language').value));
  $('#appearance').addEventListener('change', () => {
    document.body.classList.toggle('light', $('#appearance').value === 'light');
    try { localStorage.setItem('sabo-theme', $('#appearance').value); } catch (_) {}
  });
  $('#settingsPanel').addEventListener('click', event => { if (event.target === $('#settingsPanel')) hideModal('#settingsPanel'); });
  $('#queuePanel').addEventListener('click', event => { if (event.target === $('#queuePanel')) hideModal('#queuePanel'); });
  $('#checkUpdate').addEventListener('click', checkUpdate);
  $('#installUpdate').addEventListener('click', installUpdate);
  $('#installFFmpeg').addEventListener('click', () => startToolJob('#installFFmpeg', '/tools/install-ffmpeg'));
  $('#updateYtDlp').addEventListener('click', () => startToolJob('#updateYtDlp', '/tools/update-ytdlp'));
  $('#speedLimit').addEventListener('change', async () => {
    const kb = Number($('#speedLimit').value);
    try { await postAction('set-speed', { index: kb }); toast(kb === 0 ? 'Global speed limit removed' : `Global speed limit set to ${bytes(kb * 1024)}/s`); }
    catch (error) { toast(error.message, 'error'); }
  });
}

async function init() {
  try {
    if (localStorage.getItem('sabo-theme') === 'light') document.body.classList.add('light');
  } catch (_) {}
  $('#appearance').value = document.body.classList.contains('light') ? 'light' : 'dark';
  let savedLanguage = 'en';
  try { savedLanguage = localStorage.getItem('sabo-language') || 'en'; } catch (_) {}
  $('#language').value = savedLanguage;
  applyLanguage(savedLanguage);
  bindEvents();
  await refresh();
  await refreshCapabilities();
  setInterval(refresh, 1200);
}

init().catch(error => toast(error.message || 'Dashboard initialization failed', 'error'));
