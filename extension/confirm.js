const translations={en:{title:"Download with Sabo DM",subtitle:"Review the file before starting the download.",fileType:"File type",fileSize:"File size",source:"Source",download:"Download File",queue:"Add to Queue",cancel:"Cancel Download",note:"Some websites hide file size until the transfer begins. Media streams may not have a single known size.",unknown:"Unknown",expired:"This download request has expired. Close this window and try again.",cancelling:"Cancelling…",addingQueue:"Adding to queue…",sending:"Sending to Sabo DM…",cancelled:"Download cancelled.",queued:"Added to the queue.",sent:"Download added to Sabo DM."},ar:{title:"التنزيل باستخدام Sabo DM",subtitle:"راجع الملف قبل بدء التنزيل.",fileType:"نوع الملف",fileSize:"حجم الملف",source:"المصدر",download:"تنزيل الملف",queue:"إضافة إلى قائمة الانتظار",cancel:"إلغاء التنزيل",note:"قد تخفي بعض المواقع حجم الملف حتى بدء النقل. وقد لا يكون للبث حجم واحد معروف.",unknown:"غير معروف",expired:"انتهت صلاحية طلب التنزيل. أغلق النافذة وحاول مرة أخرى.",cancelling:"جارٍ الإلغاء…",addingQueue:"جارٍ الإضافة إلى قائمة الانتظار…",sending:"جارٍ الإرسال إلى Sabo DM…",cancelled:"تم إلغاء التنزيل.",queued:"تمت الإضافة إلى قائمة الانتظار.",sent:"تمت إضافة التنزيل إلى Sabo DM."},fr:{title:"Télécharger avec Sabo DM",subtitle:"Vérifiez le fichier avant de lancer le téléchargement.",fileType:"Type de fichier",fileSize:"Taille du fichier",source:"Source",download:"Télécharger le fichier",queue:"Ajouter à la file",cancel:"Annuler le téléchargement",note:"Certains sites masquent la taille jusqu'au début du transfert. Les flux multimédias peuvent ne pas avoir de taille connue.",unknown:"Inconnue",expired:"Cette demande a expiré. Fermez cette fenêtre et réessayez.",cancelling:"Annulation…",addingQueue:"Ajout à la file…",sending:"Envoi à Sabo DM…",cancelled:"Téléchargement annulé.",queued:"Ajouté à la file.",sent:"Téléchargement ajouté à Sabo DM."}};
let lang="en";
function t(k){return (translations[lang]||translations.en)[k]||translations.en[k]||k;}
function applyLanguage(){document.documentElement.lang=lang;document.documentElement.dir=lang==="ar"?"rtl":"ltr";document.querySelectorAll("[data-i18n]").forEach(el=>el.textContent=t(el.dataset.i18n));}
async function syncLanguageFromExtension(){
  const stored=await chrome.storage.local.get("extensionLanguage");
  const next=["en","ar","fr"].includes(stored.extensionLanguage)?stored.extensionLanguage:"en";
  if(next!==lang){lang=next;applyLanguage();}
}
chrome.storage.onChanged.addListener((changes,area)=>{if(area==="local"&&changes.extensionLanguage)syncLanguageFromExtension();});

const params = new URLSearchParams(location.search);
const id = params.get("id");
const statusEl = document.getElementById("status");
function cleanFilename(value, url = "") {
  let name = String(value || "").trim();
  if (name) {
    name = name.split(/[\\/]/).pop().split(/[?#]/)[0];
    try { name = decodeURIComponent(name.replace(/\+/g, "%20")); } catch (_) {}
  }
  if (!name && url) {
    try { name = new URL(url).pathname.split("/").filter(Boolean).pop() || ""; name = decodeURIComponent(name); } catch (_) {}
  }
  name = name.replace(/[\u0000-\u001f]/g, "").replace(/\s+/g, " ").trim();
  return name && name !== "." && name !== ".." ? name.slice(0,240) : "Download";
}
function cleanDisplayUrl(value) { try { const u = new URL(value); return u.origin + u.pathname; } catch (_) { return String(value || ""); } }
function humanBytes(n) { if (!n) return t("unknown"); const units=["B","KB","MB","GB","TB"]; let i=0,v=n; while(v>=1024&&i<units.length-1){v/=1024;i++;} return `${v.toFixed(i?1:0)} ${units[i]}`; }
(async () => {
  const settings = await chrome.storage.local.get(["pendingDownloads","extensionLanguage"]);
  lang=["en","ar","fr"].includes(settings.extensionLanguage)?settings.extensionLanguage:"en"; applyLanguage();
  const pendingDownloads = settings.pendingDownloads || {};
  const d = pendingDownloads[id];
  if (!d) { statusEl.textContent = t("expired"); document.querySelectorAll("button").forEach(b=>b.disabled=true); return; }
  document.getElementById("name").textContent = cleanFilename(d.name, d.url);
  document.getElementById("url").textContent = cleanDisplayUrl(d.url);
  document.getElementById("url").title = d.url || "";
  document.getElementById("type").textContent = d.type || t("unknown");
  document.getElementById("size").textContent = humanBytes(d.size);
  try { document.getElementById("source").textContent = new URL(d.referrer || d.url).hostname; } catch (_) {}
})();
async function act(action) {
  const buttons=[...document.querySelectorAll("button")]; buttons.forEach(b=>b.disabled=true); statusEl.style.color="#344054"; statusEl.textContent=action==="cancel"?t("cancelling"):action==="queue"?t("addingQueue"):t("sending");
  try {
    const result = await new Promise(resolve => chrome.runtime.sendMessage({type:"confirm-action",id,action},resolve));
    if (!result || !result.ok) throw new Error(result?.error || "Could not contact Sabo DM.");
    statusEl.style.color="#067647"; statusEl.textContent=action==="cancel"?t("cancelled"):action==="queue"?t("queued"):t("sent");
    setTimeout(()=>window.close(),650);
  } catch(e) { statusEl.style.color="#b42318"; statusEl.textContent=e.message; buttons.forEach(b=>b.disabled=false); }
}
document.getElementById("download").addEventListener("click",()=>act("download"));
document.getElementById("queue").addEventListener("click",()=>act("queue"));
document.getElementById("cancel").addEventListener("click",()=>act("cancel"));

// The confirmation popup follows the extension-specific language preference live.
