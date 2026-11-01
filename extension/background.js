// Browser integration for Sabo DM. Downloads are held until the user chooses an action.
const API = "http://127.0.0.1:19345";

const extensionText = {
  en: { action:"Sabo DM: download manager", link:"Download with Sabo DM…", video:"Download video with Sabo DM…", torrent:"Add torrent with Sabo DM…", off:"OFF", error:"ERR" },
  ar: { action:"Sabo DM: مدير التنزيلات", link:"التنزيل باستخدام Sabo DM…", video:"تنزيل الفيديو باستخدام Sabo DM…", torrent:"إضافة تورنت إلى Sabo DM…", off:"متوقف", error:"خطأ" },
  fr: { action:"Sabo DM : gestionnaire de téléchargements", link:"Télécharger avec Sabo DM…", video:"Télécharger la vidéo avec Sabo DM…", torrent:"Ajouter un torrent à Sabo DM…", off:"ARRÊT", error:"ERR" }
};
let activeLanguage = "en";
function languageText(lang=activeLanguage){return extensionText[lang]||extensionText.en;}
async function syncExtensionLanguage(){
  try {
    const stored = await chrome.storage.local.get("extensionLanguage");
    const lang = ["en","ar","fr"].includes(stored.extensionLanguage) ? stored.extensionLanguage : "en";
    activeLanguage = lang;
    const t=languageText(lang);
    await chrome.action.setTitle({title:t.action});
    for(const [id,title] of [["saber-link",t.link],["saber-video",t.video],["saber-torrent",t.torrent]]) {
      try { await chrome.contextMenus.update(id,{title}); } catch (_) {}
    }
    try { const tabs=await chrome.tabs.query({}); for(const tab of tabs) if(tab.id!=null) chrome.tabs.sendMessage(tab.id,{type:"saber-language",language:lang}).catch(()=>{}); } catch (_) {}
  } catch (_) {}
}
chrome.storage.onChanged.addListener((changes, area) => {
  if (area === "local" && changes.extensionLanguage) syncExtensionLanguage();
});

async function alive() {
  try { const r = await fetch(API + "/ping"); return r.ok && (await r.text()) === "sabo-dm"; }
  catch (_) { return false; }
}
async function cookieHeader(url) {
  try { return (await chrome.cookies.getAll({ url })).map(c => c.name + "=" + c.value).join("; "); }
  catch (_) { return ""; }
}
function cookieScopeForHost(host) {
  host = String(host || "").toLowerCase();
  // Only collect cookies for the site the user explicitly chose to download from.
  // Never enumerate all browser cookies or unrelated account sessions.
  const scopes = [
    [/^(?:[^.]+\.)*udemy\.com$/, ["udemy.com"]],
    [/^(?:[^.]+\.)*coursera\.org$/, ["coursera.org"]],
    [/^(?:[^.]+\.)*linkedin\.com$/, ["linkedin.com"]],
    [/^(?:[^.]+\.)*linkedinlearning\.com$/, ["linkedinlearning.com"]],
    [/^(?:[^.]+\.)*skillshare\.com$/, ["skillshare.com"]],
    [/^(?:[^.]+\.)*pluralsight\.com$/, ["pluralsight.com"]],
    [/^(?:[^.]+\.)*edx\.org$/, ["edx.org"]],
    [/^(?:[^.]+\.)*khanacademy\.org$/, ["khanacademy.org"]],
    [/^(?:[^.]+\.)*masterclass\.com$/, ["masterclass.com"]],
    [/^(?:[^.]+\.)*futurelearn\.com$/, ["futurelearn.com"]],
    [/^(?:[^.]+\.)*domestika\.org$/, ["domestika.org"]],
    [/^(?:[^.]+\.)*youtube\.com$|^youtu\.be$/, ["youtube.com", "google.com"]],
    [/^(?:[^.]+\.)*vimeo\.com$/, ["vimeo.com"]],
    // Social-site cookies are read only after explicit opt-in in the extension.
    [/^(?:[^.]+\.)*facebook\.com$|^(?:[^.]+\.)*fb\.watch$/, ["facebook.com", "fb.watch"]],
    [/^(?:[^.]+\.)*instagram\.com$/, ["instagram.com"]],
    [/^(?:[^.]+\.)*(?:x\.com|twitter\.com)$/, ["x.com", "twitter.com"]]
  ];
  for (const [pattern, domains] of scopes) if (pattern.test(host)) return domains;
  return [];
}
async function loginCookies(url) {
  const found = new Map();
  const add = list => list.forEach(c => found.set(c.domain + "|" + c.path + "|" + c.name, c));
  let host = "";
  try {
    const parsed = new URL(url); host = parsed.hostname;
    add(await chrome.cookies.getAll({ url: parsed.href }));
  } catch (_) {}
  for (const domain of cookieScopeForHost(host)) {
    try { add(await chrome.cookies.getAll({ domain })); } catch (_) {}
  }
  return [...found.values()]
    .filter(c => c.name && c.value && (!c.expirationDate || c.expirationDate * 1000 > Date.now()))
    .map(c => ({ domain:c.domain, path:c.path, name:c.name, value:c.value, secure:!!c.secure, httpOnly:!!c.httpOnly, expires:c.expirationDate||0 }));
}
function flash(text, color) {
  chrome.action.setBadgeBackgroundColor({ color }); chrome.action.setBadgeText({ text });
  setTimeout(() => chrome.action.setBadgeText({ text: "" }), 2500);
}
function normalizeSocialUrl(value) {
  let current = String(value || "").trim();
  for (let i = 0; i < 4; i++) {
    let u; try { u = new URL(current); } catch (_) { return current; }
    const host = u.hostname.toLowerCase(), path = u.pathname.toLowerCase();
    const social = /(^|\.)facebook\.com$/.test(host) || /(^|\.)instagram\.com$/.test(host) || /(^|\.)(x|twitter)\.com$/.test(host);
    if (!social || !/(login|redirect|checkpoint|oauth)/i.test(path)) return current;
    let next = "";
    for (const key of ["next", "continue", "redirect", "redirect_uri", "url", "u", "return_url"]) { const v = u.searchParams.get(key); if (v) { next = v; break; } }
    if (!next || next === current) return current;
    try { next = decodeURIComponent(next); } catch (_) {}
    if (!/^https?:\/\//i.test(next)) return current;
    current = next;
  }
  return current;
}
function cleanFilename(value, url = "") {
  // Prefer a real browser-supplied filename, but strip path/query fragments and
  // decode URL-escaped names such as %5BCima%5D%20Movie.mp4.
  let raw = String(value || "").trim();
  if (!raw || /^(video|audio|media file|download)$/i.test(raw)) raw = "";
  if (raw) {
    raw = raw.split(/[\\/]/).pop().split(/[?#]/)[0];
    try { raw = decodeURIComponent(raw.replace(/\+/g, "%20")); } catch (_) {}
  }
  if (!raw && url) {
    try {
      const u = new URL(url);
      raw = u.pathname.split("/").filter(Boolean).pop() || "";
      try { raw = decodeURIComponent(raw); } catch (_) {}
    } catch (_) {}
  }
  raw = raw.replace(/[\u0000-\u001f]/g, "").replace(/\s+/g, " ").trim();
  // A URL with no useful final path segment is not a filename.
  if (!raw || raw === "." || raw === "..") return "Download";
  return raw.slice(0, 240);
}
function displayUrl(value) {
  try { const u = new URL(value); return u.origin + u.pathname; } catch (_) { return String(value || ""); }
}
function extType(name, mime) {
  if (mime && mime !== "application/octet-stream") return mime;
  let path = String(name || "").split(/[?#]/)[0];
  try { path = decodeURIComponent(path); } catch (_) {}
  const ext = path.split(".").pop().toLowerCase();
  const types = { mp4:"Video (MP4)",mkv:"Video (MKV)",webm:"Video (WebM)",mp3:"Audio (MP3)",m4a:"Audio (M4A)",wav:"Audio (WAV)",pdf:"PDF document",zip:"ZIP archive",rar:"RAR archive",exe:"Windows application",msi:"Windows installer",jpg:"Image (JPEG)",png:"Image (PNG)",torrent:"Torrent file" };
  return types[ext] || (ext ? ext.toUpperCase() + " file" : "Unknown file type");
}
async function openConfirm(data) {
  const id = crypto.randomUUID();
  const pending = (await chrome.storage.local.get("pendingDownloads")).pendingDownloads || {};
  pending[id] = { ...data, createdAt: Date.now() };
  await chrome.storage.local.set({ pendingDownloads: pending });
  await chrome.windows.create({ url: chrome.runtime.getURL("confirm.html?id=" + encodeURIComponent(id)), type: "popup", width: 520, height: 570, focused: true });
}
async function send(data, action) {
  const { useLogin = false } = await chrome.storage.local.get("useLogin");
  const kind = data.kind || "";
  const cookie = kind === "video" || kind === "torrent" ? "" : await cookieHeader(data.url);
  const cookies = (kind === "video" || kind === "audio") && useLogin ? await loginCookies(data.url) : [];
  if ((kind === "video" || kind === "audio") && useLogin && cookieScopeForHost(new URL(data.url).hostname).length && cookies.length === 0) {
    throw new Error("No active site cookies were found. Sign in to this learning platform in the same browser, refresh the course page, and try again. Your password is never requested or collected by Sabo DM.");
  }
  const r = await fetch(API + "/add", {
    method: "POST", headers: { "Content-Type": "application/json", "X-Sabo-DM": "1" },
    body: JSON.stringify({ url:data.url, referrer:data.referrer||"", cookie, cookies, kind, action, audioOnly:!!data.audioOnly, name:cleanFilename(data.name, data.url), size:data.size||0 })
  });
  if (!r.ok) throw new Error(await r.text().catch(() => "Sabo DM rejected the download"));
}

// Hold regular browser downloads while the user decides. If Sabo DM is unavailable,
// leave the browser download alone so the user is not unexpectedly blocked.
chrome.downloads.onCreated.addListener(async item => {
  const { enabled = true } = await chrome.storage.local.get("enabled");
  if (!enabled || item.state !== "in_progress") return;
  const url = normalizeSocialUrl(item.finalUrl || item.url);
  if (!/^https?:/i.test(url) || !(await alive())) return;
  try { await chrome.downloads.pause(item.id); } catch (_) {}
  try {
    await openConfirm({ source:"browser", downloadId:item.id, url, referrer:item.referrer||"", name:item.filename?.split(/[\\/]/).pop() || url.split("/").pop() || "download", size:item.totalBytes||item.fileSize||0, mime:item.mime||"", type:extType(item.filename, item.mime), kind:/\.torrent(?:$|\?)/i.test(url) ? "torrent" : "" });
  } catch (_) { try { await chrome.downloads.resume(item.id); } catch (_) {} }
});

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({ id:"saber-link", title:"Download with Sabo DM…", contexts:["link","audio"] });
  chrome.contextMenus.create({ id:"saber-video", title:"Download video with Sabo DM…", contexts:["page","video"] });
  chrome.contextMenus.create({ id:"saber-torrent", title:"Add torrent with Sabo DM…", contexts:["link"] });
  chrome.storage.local.get("extensionLanguage").then(v => { if (!v.extensionLanguage) return chrome.storage.local.set({extensionLanguage:"en"}); }).finally(syncExtensionLanguage);
});
chrome.runtime.onStartup.addListener(() => { chrome.storage.local.get("extensionLanguage").then(v => { if (!v.extensionLanguage) return chrome.storage.local.set({extensionLanguage:"en"}); }).finally(syncExtensionLanguage); });
chrome.contextMenus.onClicked.addListener(async (info) => {
  const isVideo = info.menuItemId === "saber-video";
  const isTorrent = info.menuItemId === "saber-torrent";
  const url = normalizeSocialUrl(isVideo ? (info.srcUrl || info.linkUrl || info.pageUrl) : (info.linkUrl || info.srcUrl));
  if (!url || !/^(https?:|magnet:)/i.test(url)) return;
  if (!(await alive())) { flash(languageText().off, "#b91c1c"); return; }
  const kind = /^magnet:/i.test(url) || isTorrent ? "torrent" : isVideo ? "video" : "";
  let name = cleanFilename("", url);
  try { await openConfirm({ source:"menu", url, referrer:info.pageUrl||"", name:name|| (kind === "video" ? "Video" : kind === "torrent" ? "Torrent" : "Download"), size:0, type:extType(name, ""), kind }); }
  catch (_) { flash(languageText().error, "#b91c1c"); }
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg || msg.type !== "video") return;
  (async () => {
    if (!(await alive())) throw new Error("Start Sabo DM first");
    const normalizedUrl = normalizeSocialUrl(msg.url);
    const name = cleanFilename(msg.name || "", normalizedUrl);
    await openConfirm({ source:"content", url:normalizedUrl, referrer:msg.referrer||"", name, size:0, type:msg.mediaType||"Video or audio stream", kind:msg.direct ? "" : (msg.audioOnly ? "audio" : "video"), audioOnly:!!msg.audioOnly });
    sendResponse({ ok:true });
  })().catch(e => sendResponse({ ok:false, error:e.message }));
  return true;
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg || msg.type !== "confirm-action") return;
  (async () => {
    const stored = await chrome.storage.local.get("pendingDownloads");
    const pending = stored.pendingDownloads || {};
    const data = pending[msg.id];
    if (!data) throw new Error("This download request has expired.");
    if (msg.action === "cancel") {
      if (data.downloadId != null) { try { await chrome.downloads.cancel(data.downloadId); } catch (_) {} try { await chrome.downloads.erase({ id:data.downloadId }); } catch (_) {} }
      delete pending[msg.id]; await chrome.storage.local.set({ pendingDownloads:pending });
      sendResponse({ ok:true }); return;
    }
    await send(data, msg.action === "queue" ? "queue" : "download");
    if (data.downloadId != null) { try { await chrome.downloads.cancel(data.downloadId); } catch (_) {} try { await chrome.downloads.erase({ id:data.downloadId }); } catch (_) {} }
    delete pending[msg.id]; await chrome.storage.local.set({ pendingDownloads:pending });
    flash(msg.action === "queue" ? "Q" : "OK", "#15803d");
    sendResponse({ ok:true });
  })().catch(e => sendResponse({ ok:false, error:e.message }));
  return true;
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg?.type === "get-saber-language") { chrome.storage.local.get("extensionLanguage").then(v => { activeLanguage = ["en","ar","fr"].includes(v.extensionLanguage) ? v.extensionLanguage : "en"; sendResponse({language:activeLanguage}); }); return true; }
});
