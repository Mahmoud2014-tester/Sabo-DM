const translations={
 en:{tagline:"DOWNLOAD MANAGER",title:"Download with Sabo DM",subtitle:"Choose how this file should be handled.",type:"File type",size:"File size",source:"Source",audioFormat:"Audio output format",output:"Download format",download:"Download now",queue:"Add to queue",cancel:"Cancel browser download",note:"The original browser download is only cancelled after Sabo DM confirms that it accepted the request.",unknown:"Unknown",expired:"This request expired. Close this window and try again from the browser.",loading:"Loading download…",sending:"Sending request to Sabo DM…",queued:"Accepted by Sabo DM and added to the queue.",sent:"Accepted by Sabo DM. The download is now in the desktop queue.",cancelled:"Browser download cancelled.",ffmpeg:"FFmpeg is installed. Audio conversion formats are available.",noFFmpeg:"FFmpeg is not installed. Install it in Sabo DM Settings to convert audio. Original audio remains available.",engineOffline:"Sabo DM is not running. Start the desktop application and retry.",error:"The desktop application did not accept this download."},
 ar:{tagline:"مدير التنزيلات",title:"التنزيل باستخدام Sabo DM",subtitle:"اختر الإجراء المطلوب لهذا الملف.",type:"نوع الملف",size:"حجم الملف",source:"المصدر",audioFormat:"صيغة الصوت الناتج",output:"صيغة التنزيل",download:"تنزيل الآن",queue:"إضافة إلى قائمة الانتظار",cancel:"إلغاء تنزيل المتصفح",note:"لن يُلغى تنزيل المتصفح الأصلي إلا بعد تأكيد قبول الطلب داخل Sabo DM.",unknown:"غير معروف",expired:"انتهت صلاحية الطلب. أغلق النافذة وحاول مجددًا من المتصفح.",loading:"جارٍ تحميل بيانات التنزيل…",sending:"جارٍ إرسال الطلب إلى Sabo DM…",queued:"تم قبول الطلب وإضافته إلى قائمة الانتظار.",sent:"تم قبول الطلب وإضافته إلى قائمة التنزيلات في التطبيق.",cancelled:"تم إلغاء تنزيل المتصفح.",ffmpeg:"FFmpeg مثبت. صيغ تحويل الصوت متاحة.",noFFmpeg:"FFmpeg غير مثبت. ثبّته من إعدادات Sabo DM لتحويل الصوت. يمكنك الاحتفاظ بالصيغة الأصلية.",engineOffline:"تطبيق Sabo DM غير مشغّل. شغّل التطبيق ثم حاول مجددًا.",error:"لم يقبل التطبيق طلب التنزيل."},
 fr:{tagline:"GESTIONNAIRE DE TÉLÉCHARGEMENT",title:"Télécharger avec Sabo DM",subtitle:"Choisissez l’action pour ce fichier.",type:"Type de fichier",size:"Taille",source:"Source",audioFormat:"Format audio de sortie",output:"Format de téléchargement",download:"Télécharger maintenant",queue:"Ajouter à la file",cancel:"Annuler le téléchargement du navigateur",note:"Le téléchargement du navigateur ne sera annulé qu’après confirmation de Sabo DM.",unknown:"Inconnu",expired:"Cette demande a expiré. Fermez cette fenêtre et réessayez depuis le navigateur.",loading:"Chargement…",sending:"Envoi à Sabo DM…",queued:"Demande acceptée et ajoutée à la file.",sent:"Demande acceptée dans la file de Sabo DM.",cancelled:"Téléchargement du navigateur annulé.",ffmpeg:"FFmpeg est installé. La conversion audio est disponible.",noFFmpeg:"FFmpeg n’est pas installé. Installez-le dans les paramètres de Sabo DM pour convertir l’audio.",engineOffline:"Sabo DM n’est pas lancé. Démarrez l’application puis réessayez.",error:"L’application n’a pas accepté ce téléchargement."}
};
const $=id=>document.getElementById(id);let lang="en",requestId="",pending=null,capabilities=null;
function t(k){return (translations[lang]||translations.en)[k]||translations.en[k]||k}
function applyLanguage(){document.documentElement.lang=lang;document.documentElement.dir=lang==="ar"?"rtl":"ltr";document.querySelectorAll('[data-i18n]').forEach(el=>el.textContent=t(el.dataset.i18n));document.title="Sabo DM — Download";}
function cleanFilename(value,url=""){let name=String(value||"").trim();if(name){name=name.split(/[\\/]/).pop().split(/[?#]/)[0];try{name=decodeURIComponent(name.replace(/\+/g,"%20"))}catch(_){}}if(!name&&url){try{name=new URL(url).pathname.split('/').filter(Boolean).pop()||"";name=decodeURIComponent(name)}catch(_){}}name=name.replace(/[\u0000-\u001f]/g,"").replace(/\s+/g," ").trim();return name&&name!=="."&&name!==".."?name.slice(0,240):"Download"}
function humanBytes(n){if(!n)return t("unknown");const u=["B","KB","MB","GB","TB"];let i=0,v=n;while(v>=1024&&i<u.length-1){v/=1024;i++}return `${v.toFixed(i?1:0)} ${u[i]}`}
function showStatus(message,kind=""){const el=$('status');el.className="status "+kind;el.textContent=message}
function sendMessage(message){return new Promise((resolve,reject)=>chrome.runtime.sendMessage(message,response=>{const err=chrome.runtime.lastError;if(err)return reject(new Error(err.message));resolve(response)}))}
async function init(){requestId=new URLSearchParams(location.search).get("id")||"";const key="pendingDownload_"+requestId;const saved=await chrome.storage.local.get([key,"pendingDownloads","extensionLanguage"]);pending=saved[key]||saved.pendingDownloads?.[requestId]||null;lang=["en","ar","fr"].includes(saved.extensionLanguage)?saved.extensionLanguage:"en";applyLanguage();if(!pending){showStatus(t("expired"),"error");document.querySelectorAll('button').forEach(b=>b.disabled=true);return;}
$('name').textContent=cleanFilename(pending.name,pending.url);try{$('url').textContent=new URL(pending.url).origin+new URL(pending.url).pathname;$('source').textContent=new URL(pending.referrer||pending.url).hostname}catch(_){$('url').textContent=pending.url||""}
$('type').textContent=pending.type||t("unknown");$('size').textContent=humanBytes(pending.size);const isAudio=pending.kind==="audio"||pending.audioOnly||/^audio/i.test(pending.type||"");
const mediaPage = pending.kind==="video" || pending.kind==="audio";
$('mediaOutput').value=isAudio?"original":"video";
try{capabilities=await sendMessage({type:"engine-capabilities"});}catch(_){capabilities={ffmpegReady:false};}
if(!capabilities?.ok)capabilities={ffmpegReady:false};
for(const option of $('mediaOutput').options){
  if(option.value==="video"){option.disabled=isAudio;continue;}
  if(option.value==="original"){option.disabled=false;continue;}
  option.disabled=!mediaPage || !capabilities.ffmpegReady;
}
if(!mediaPage){
  $('formatNote').textContent=isAudio?"This is a direct audio file. Sabo DM will preserve its original format.":"This is a direct file URL. Audio extraction is available for supported media pages.";
}else{
  $('formatNote').textContent=capabilities.ffmpegReady?t("ffmpeg"):t("noFFmpeg");
}
}
async function act(action){if(!pending)return;const buttons=[...document.querySelectorAll('button')];buttons.forEach(b=>b.disabled=true);showStatus(t("sending"));try{const selected=$('mediaOutput').value||"video";
const audioOnly=selected!=="video";
const audioFormat=audioOnly?selected:"original";
const result=await sendMessage({type:"confirm-action",id:requestId,action,audioOnly,audioFormat});if(!result?.ok)throw new Error(result?.error||t("error"));showStatus(action==="cancel"?t("cancelled"):action==="queue"?t("queued"):t("sent"),"ok");setTimeout(()=>window.close(),700);}catch(error){showStatus(error.message||t("error"),"error");buttons.forEach(b=>b.disabled=false)}}
$('download').addEventListener('click',()=>act("download"));$('queue').addEventListener('click',()=>act("queue"));$('cancel').addEventListener('click',()=>act("cancel"));
init().catch(error=>showStatus(error.message||t("error"),"error"));
