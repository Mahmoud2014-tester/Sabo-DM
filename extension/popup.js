const i18n={
 en:{takeover:"Take over browser downloads",login:"Use my login for videos",hint:"When enabled, cookies for supported video, learning and social sites (Facebook, Instagram and X) are sent to Sabo DM on this computer. Off by default.",checking:"Checking…",connected:"App connected",offline:"App not running",languageLabel:"Extension language",languageSource:"Extension language is independent from desktop language."},
 ar:{takeover:"إدارة تنزيلات المتصفح",login:"استخدام تسجيل دخولي للفيديوهات",hint:"عند التفعيل، تُرسل ملفات تعريف الارتباط لمواقع الفيديو والتعلّم والتواصل المدعومة (Facebook وInstagram وX) إلى Sabo DM على هذا الكمبيوتر. الخيار معطّل افتراضيًا.",checking:"جارٍ التحقق…",connected:"التطبيق متصل",offline:"التطبيق غير مشغّل",languageLabel:"لغة الإضافة",languageSource:"لغة الإضافة مستقلة عن لغة تطبيق سطح المكتب."},
 fr:{takeover:"Gérer les téléchargements du navigateur",login:"Utiliser ma session pour les vidéos",hint:"Activés, les cookies des sites vidéo, d’apprentissage et sociaux pris en charge (Facebook, Instagram et X) sont transmis à Sabo DM sur cet ordinateur. Désactivé par défaut.",checking:"Vérification…",connected:"Application connectée",offline:"Application non démarrée",languageLabel:"Langue de l’extension",languageSource:"La langue de l’extension est indépendante de celle de l’application."}
};
const box=document.getElementById("enabled");
const login=document.getElementById("useLogin");
const status=document.getElementById("status");
const languageSelect=document.getElementById("extensionLanguage");
let currentLang="en";
function applyLanguage(lang){currentLang=["en","ar","fr"].includes(lang)?lang:"en";const t=i18n[currentLang];document.documentElement.lang=currentLang;document.documentElement.dir=currentLang==="ar"?"rtl":"ltr";document.querySelectorAll("[data-i18n]").forEach(el=>el.textContent=t[el.dataset.i18n]||"");}
async function syncLanguage(){
 const stored=await chrome.storage.local.get("extensionLanguage");
 currentLang=["en","ar","fr"].includes(stored.extensionLanguage)?stored.extensionLanguage:"en";
 languageSelect.value=currentLang; applyLanguage(currentLang);
 try{const r=await fetch("http://127.0.0.1:19345/ping",{cache:"no-store"});if(!r.ok)throw new Error("offline");status.textContent=i18n[currentLang].connected;status.className="ok";}
 catch(_){status.textContent=i18n[currentLang].offline;status.className="off";}
}
languageSelect.addEventListener("change",async()=>{currentLang=languageSelect.value;await chrome.storage.local.set({extensionLanguage:currentLang});applyLanguage(currentLang);status.textContent=i18n[currentLang].checking;syncLanguage();});
chrome.storage.onChanged.addListener((changes,area)=>{if(area==="local"&&changes.extensionLanguage)syncLanguage();});
chrome.storage.local.get(["enabled","useLogin"]).then(({enabled=true,useLogin=false})=>{box.checked=enabled;login.checked=useLogin;});
box.addEventListener("change",()=>chrome.storage.local.set({enabled:box.checked}));
login.addEventListener("change",()=>chrome.storage.local.set({useLogin:login.checked}));
syncLanguage();
