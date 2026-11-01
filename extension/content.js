// Shows a round download button on video players. It also works in full screen
// and inside embedded players.
(function () {
  const ICON = "\u2B07";
  const AUDIO_ICON = "♫";
  const labels = {
    en: {media:"⬇  Download media", video:"Download video with Sabo DM", audio:"Download audio with Sabo DM", sending:"Sending…", sent:"✓  Sent to Sabo DM", failed:"✗  Is Sabo DM running?", noPost:"✗  Could not resolve this post or a direct video URL. Use the post menu → Copy link."},
    ar: {media:"⬇  تنزيل الوسائط", video:"تنزيل الفيديو باستخدام Sabo DM", audio:"تنزيل الصوت باستخدام Sabo DM", sending:"جارٍ الإرسال…", sent:"✓  تم الإرسال إلى Sabo DM", failed:"✗  هل Sabo DM يعمل؟", noPost:"✗  تعذر تحديد رابط المنشور أو الفيديو. استخدم قائمة المنشور ثم نسخ الرابط."},
    fr: {media:"⬇  Télécharger le média", video:"Télécharger la vidéo avec Sabo DM", audio:"Télécharger l’audio avec Sabo DM", sending:"Envoi…", sent:"✓  Envoyé à Sabo DM", failed:"✗  Sabo DM est-il lancé ?", noPost:"✗  Publication ou flux vidéo introuvable. Utilisez Copier le lien dans le menu."}
  };
  let language = "en";
  const text = () => labels[language] || labels.en;
  function setLanguage(lang) { if (labels[lang] && lang !== language) { language = lang; if (btn && !busy) btn.textContent = lastMediaType === "audio" ? AUDIO_ICON : ICON; update(); } }
  chrome.runtime.onMessage.addListener(msg => { if (msg?.type === "saber-language") setLanguage(msg.language); });
  chrome.runtime.sendMessage({type:"get-saber-language"}, response => { void chrome.runtime.lastError; if (response?.language) setLanguage(response.language); });
  
  const usePopover = "popover" in HTMLElement.prototype;
  let btn = null;
  let busy = false;
  let lastMediaType = "video";
  let buttonMediaElement = null;
  let hideTimer = null;

  function unwrapSocialRedirect(value) {
    let current = String(value || "");
    for (let i = 0; i < 4; i++) {
      let u; try { u = new URL(current, location.href); } catch (_) { return current; }
      const host = u.hostname.toLowerCase(), path = u.pathname.toLowerCase();
      const social = /(^|\.)facebook\.com$/.test(host) || /(^|\.)instagram\.com$/.test(host) || /(^|\.)(x|twitter)\.com$/.test(host);
      if (!social || !/(login|redirect|checkpoint|oauth)/i.test(path)) return u.href;
      let next = "";
      for (const key of ["next", "continue", "redirect", "redirect_uri", "url", "u", "return_url"]) { const v = u.searchParams.get(key); if (v) { next = v; break; } }
      if (!next) return u.href;
      try { next = decodeURIComponent(next); } catch (_) {}
      if (!/^https?:\/\//i.test(next)) return u.href;
      current = next;
    }
    return current;
  }

  function soundCloudTrackUrl() {
    // Resolve the track that is actually loaded in SoundCloud's global player.
    // Do not choose the first arbitrary track link on a profile/playlist page.
    const host = location.hostname.toLowerCase();
    if (!/(^|\.)soundcloud\.com$/.test(host)) return "";
    const candidates = [];
    const add = value => { if (value) candidates.push(String(value)); };

    // SoundCloud's now-playing badge is the most reliable source on profile,
    // stream and playlist pages. Prefer its title link over page-wide anchors.
    const activeSelectors = [
      '.playbackSoundBadge__titleLink',
      '.playControls__soundBadge a[href]',
      '.playbackSoundBadge a[href]',
      '[data-testid="playbackSoundBadge"] a[href]',
      '.playControls__soundBadge',
      '.playbackSoundBadge'
    ];
    for (const selector of activeSelectors) {
      for (const el of document.querySelectorAll(selector)) {
        add(el.href || el.getAttribute('href'));
        const link = el.querySelector?.('a[href]');
        if (link) add(link.href);
      }
    }

    // Track link associated with the currently playing list item.
    for (const selector of ['.soundList__item.sc-pause a[href]', '.soundList__item.playing a[href]', '.sc-classic .playing a[href]', '[aria-current="true"][href*="soundcloud.com/"]']) {
      for (const el of document.querySelectorAll(selector)) add(el.href);
    }

    // Embedded player URLs explicitly identify the embedded track.
    try {
      const embeddedTrack = new URL(location.href).searchParams.get('url');
      if (embeddedTrack) add(embeddedTrack);
      for (const frame of document.querySelectorAll('iframe[src*="soundcloud.com/player"]')) {
        const track = new URL(frame.src, location.href).searchParams.get('url');
        if (track) add(track);
      }
    } catch (_) {}

    // On an actual track page, the current page URL is authoritative.
    if (/^\/(?!player\/?$)[^/]+\/[^/]+\/?$/.test(location.pathname)) add(location.href);

    // Only inspect generic anchors as a last resort on a track-specific page;
    // never select an unrelated first track from a profile or search results.
    const canonical = document.querySelector('link[rel="canonical"]')?.href || document.querySelector('meta[property="og:url"]')?.content;
    if (canonical && /^\/(?!player\/?$)[^/]+\/[^/]+\/?$/.test(new URL(canonical, location.href).pathname)) add(canonical);

    for (const value of candidates) {
      try {
        const u = new URL(value, location.href);
        if (!/(^|\.)soundcloud\.com$/.test(u.hostname)) continue;
        if (!/^\/[^/]+\/[^/]+\/?$/.test(u.pathname)) continue;
        if (/^\/(?:discover|you|stream|search|player)\//i.test(u.pathname)) continue;
        u.search = ""; u.hash = "";
        return u.href;
      } catch (_) {}
    }
    return "";
  }


  function socialPlatform() {
    const h = location.hostname.toLowerCase();
    if (/(^|\.)facebook\.com$/.test(h) || /(^|\.)fb\.watch$/.test(h)) return "facebook";
    if (/(^|\.)instagram\.com$/.test(h)) return "instagram";
    if (/(^|\.)(x|twitter)\.com$/.test(h)) return "x";
    return "";
  }

  function isSocialPostUrl(value, platform) {
    try {
      const u = new URL(value, location.href), path = u.pathname.toLowerCase();
      const host = u.hostname.toLowerCase();
      const sameSite = u.origin === location.origin ||
        (platform === "facebook" && /(^|\.)fb\.watch$/.test(host));
      if (!sameSite) return false;
      if (platform === "x") return /^\/[^/]+\/status\/\d+/.test(path) || /^\/i\/web\/status\/\d+/.test(path);
      if (platform === "instagram") return /^\/(?:reel|reels|p|tv)\/[a-z0-9_-]+/i.test(path);
      if (platform === "facebook") {
        return /\/(?:watch|reel|reels|videos?)\/?/i.test(path) ||
          /\/posts\/[a-z0-9_-]+/i.test(path) ||
          /\/(?:permalink|story)\.php$/i.test(path) &&
            (u.searchParams.has("story_fbid") || u.searchParams.has("id")) ||
          u.searchParams.has("story_fbid") ||
          u.searchParams.has("v") && /\/watch\/?$/i.test(path) ||
          /\/share\/(?:v|r|p)\/[a-z0-9_-]+/i.test(path) ||
          /\/share\/[a-z0-9_-]+/i.test(path);
      }
    } catch (_) {}
    return false;
  }

  function facebookVideoIdFromContainer(el) {
    const keys = ["data-video-id", "data-videoid", "data-ft", "data-store", "data-video"];
    for (let node = el, depth = 0; node && depth < 12; node = node.parentElement, depth++) {
      for (const key of keys) {
        const raw = node.getAttribute?.(key);
        if (!raw) continue;
        if (/^\d{5,}$/.test(raw) && /video/i.test(key)) return raw;
        try {
          const obj = JSON.parse(raw);
          const stack = [obj];
          while (stack.length) {
            const item = stack.pop();
            if (!item || typeof item !== "object") continue;
            for (const [k,v] of Object.entries(item)) {
              if (typeof v === "string" || typeof v === "number") {
                if (/(video|story|post)/i.test(k) && /(id|key)/i.test(k) && /^\d{5,}$/.test(String(v))) return String(v);
              } else if (v && typeof v === "object") stack.push(v);
            }
          }
        } catch (_) {
          const m = raw.match(/(?:video_id|videoID|story_fbid|mf_story_key)["'\s:=]+(\d{5,})/i);
          if (m) return m[1];
        }
      }
    }
    return "";
  }

  function socialPostPermalink(el) {
    const platform = socialPlatform();
    if (!platform) return "";

    const selectors = platform === "facebook"
      ? 'a[href*="/posts/"],a[href*="/permalink.php"],a[href*="/story.php"],a[href*="/watch"],a[href*="/videos/"],a[href*="/reel"],a[href*="/share/"],a[href*="story_fbid="]'
      : platform === "instagram"
        ? 'a[href*="/reel/"],a[href*="/reels/"],a[href*="/p/"],a[href*="/tv/"]'
        : 'a[href*="/status/"]';

    const roots = [];
    if (el) {
      for (let node = el, depth = 0; node && depth < 18; node = node.parentElement, depth++) {
        if (node.matches?.('article,[role="article"],[data-pagelet*="FeedUnit"],[data-testid*="tweet"],[data-testid*="post-container"]')) roots.push(node);
        // Even when Facebook omits semantic article roles, inspect progressively
        // larger ancestors until one contains a candidate post permalink.
        if (depth >= 2 && node.querySelector?.(selectors)) roots.push(node);
      }
    }

    const scoreLinks = (scope, localOnly) => {
      const result = [];
      if (!scope?.querySelectorAll) return result;
      const mediaRect = el?.getBoundingClientRect?.();
      for (const a of scope.querySelectorAll(selectors)) {
        let u;
        try { u = new URL(a.href, location.href); } catch (_) { continue; }
        if (!isSocialPostUrl(u.href, platform)) continue;
        const path = u.pathname.toLowerCase();
        let score = 0;
        if (platform === "x") score += /\/status\/\d+/.test(path) ? 35 : 0;
        if (platform === "instagram") score += /^\/(?:reel|reels)\//.test(path) ? 45 : /^\/(?:p|tv)\//.test(path) ? 35 : 0;
        if (platform === "facebook") {
          if (/\/(?:watch|reel|reels|videos?)\//.test(path)) score += 45;
          if (/\/posts\//.test(path)) score += 40;
          if (/\/(?:permalink|story)\.php$/.test(path) || /story_fbid/.test(u.search)) score += 38;
          if (/\/share\//.test(path)) score += 30;
        }
        const r = a.getBoundingClientRect?.();
        if (mediaRect && r && (r.width || r.height)) {
          const dx = (r.left+r.right)/2 - (mediaRect.left+mediaRect.right)/2;
          const dy = (r.top+r.bottom)/2 - (mediaRect.top+mediaRect.bottom)/2;
          const distance = Math.sqrt(dx*dx + dy*dy);
          score += Math.max(0, 220 - distance/5);
        }
        if (/^(?:like|comment|share|reply|see more)$/i.test((a.innerText||"").trim())) score -= 80;
        if (a.closest?.('[aria-label*="comment" i],[aria-label*="reaction" i]')) score -= 60;
        if (localOnly) score += 35;
        result.push({url:u.href,score});
      }
      return result;
    };

    // First search the closest real post container.
    const seenRoots = new Set();
    for (const root of roots) {
      if (seenRoots.has(root)) continue;
      seenRoots.add(root);
      const candidates = scoreLinks(root, true).sort((a,b)=>b.score-a.score);
      if (candidates.length && candidates[0].score >= 70) return unwrapSocialRedirect(candidates[0].url);
    }

    // Feed DOMs change frequently. A global candidate scan with geometric
    // proximity is a fallback when the player is nested in anonymous divs.
    const globalCandidates = scoreLinks(document, false).sort((a,b)=>b.score-a.score);
    if (globalCandidates.length && globalCandidates[0].score >= 75) return unwrapSocialRedirect(globalCandidates[0].url);

    if (platform === "facebook" && el) {
      const id = facebookVideoIdFromContainer(el);
      if (id) return "https://www.facebook.com/watch/?v=" + encodeURIComponent(id);
    }

    if (isSocialPostUrl(location.href, platform)) return location.href;
    const canonical = document.querySelector('link[rel="canonical"]')?.href ||
      document.querySelector('meta[property="og:url"]')?.content;
    if (canonical && isSocialPostUrl(canonical, platform)) return unwrapSocialRedirect(canonical);
    return "";
  }

  function mediaPageUrl(el) {
    const sc = soundCloudTrackUrl();
    if (sc) return sc;
    const platform = socialPlatform();
    if (platform) {
      const post = socialPostPermalink(el);
      if (post) return unwrapSocialRedirect(post);
    }
    const src = el?.currentSrc || el?.src || "";
    if (/^https?:/i.test(src) && /\.(?:mp4|webm|m4v|mov|mp3|m4a|aac|ogg|wav)(?:$|[?#])/i.test(src)) return src;
    if (!platform) {
      const root = el?.closest?.('article, [role="article"]') || el?.parentElement?.parentElement?.parentElement || document;
      const patterns = [/\/(?:[^/]+\/status\/\d+)/i, /\/(?:reel|reels|videos?|watch)\b/i, /\/(?:p|tv)\/[A-Za-z0-9_-]+/i, /\/(?:shorts|live)\/[A-Za-z0-9_-]+/i];
      for (const a of root.querySelectorAll("a[href]")) {
        try { const u = new URL(a.href, location.href); if (u.origin === location.origin && patterns.some(re => re.test(u.pathname))) return u.href; } catch (_) {}
      }
      if (/\/(?:[^/]+\/status\/\d+|reel\/|reels\/|videos?\/|watch\/|p\/|shorts\/|live\/)/i.test(location.pathname) || /[?&]v=\d+/.test(location.search)) return location.href;
      return unwrapSocialRedirect(location.href);
    }
    // Do not send a social feed URL (home/timeline/explore) to yt-dlp as a video URL.
    return "";
  }

  let lastHoveredMedia = null;
  document.addEventListener("pointermove", e => {
    const media = e.target?.closest?.("video, audio");
    if (media) lastHoveredMedia = media;
  }, {passive:true, capture:true});

  function mediaVisible(el) {
    if (!el || !el.isConnected) return false;
    const r = el.getBoundingClientRect(), st = getComputedStyle(el);
    return r.width > 0 && r.height > 0 && r.bottom >= 0 && r.top <= innerHeight && r.right >= 0 && r.left <= innerWidth &&
      st.visibility !== "hidden" && st.display !== "none" && Number(st.opacity || 1) > 0.02;
  }

  function biggestMedia() {
    const all = Array.from(document.querySelectorAll("video, audio")).filter(mediaVisible);
    const playing = all.filter(el => !el.paused && !el.ended && (el.currentTime > 0 || el.readyState >= 2));
    if (playing.length) {
      playing.sort((a,b) => { const ar=a.getBoundingClientRect(), br=b.getBoundingClientRect(); return br.width*br.height-ar.width*ar.height; });
      const el=playing[0], r=el.getBoundingClientRect();
      return {rect:r, type:el.tagName==="AUDIO"?"audio":"video", element:el};
    }
    if (lastHoveredMedia && all.includes(lastHoveredMedia)) {
      const el=lastHoveredMedia, r=el.getBoundingClientRect();
      return {rect:r, type:el.tagName==="AUDIO"?"audio":"video", element:el};
    }
    let best=null, area=0;
    for (const el of all) {
      const r=el.getBoundingClientRect(), audio=el.tagName==="AUDIO", a=r.width*r.height;
      if ((audio ? r.width>=80 && r.height>=24 : r.width>=240 && r.height>=135) && a>area) {
        best={rect:r,type:audio?"audio":"video",element:el}; area=a;
      }
    }
    if (best) return best;
    if (/(^|\.)soundcloud\.com$/i.test(location.hostname) && soundCloudTrackUrl()) {
      const selectors=['.playControls__elements','[data-testid="playback-controls"]','.playControls__inner .playControls__elements','.playbackSoundBadge','.playControls__soundBadge','.playControls'];
      const candidates=[];
      for (const selector of selectors) {
        for (const player of document.querySelectorAll(selector)) {
          const r=player.getBoundingClientRect();
          if (r.width>=90 && r.height>=18 && r.bottom>=0 && r.top<=innerHeight) {
            const compact=r.width<=620 && r.height<=110;
            candidates.push({rect:r,compact,area:r.width*r.height,type:"audio",element:null,soundcloud:true});
          }
        }
        if (candidates.some(x=>x.compact)) break;
      }
      if (candidates.length) { candidates.sort((a,b)=>(Number(b.compact)-Number(a.compact)) || a.area-b.area); return candidates[0]; }
    }
    return null;
  }


  function isDirectSocialMediaURL(value, platform) {
    try {
      const u = new URL(value);
      const host = u.hostname.toLowerCase();
      const allowed = platform === "facebook"
        ? /(^|\.)fbcdn\.net$|(^|\.)fbsbx\.com$/.test(host)
        : platform === "instagram"
          ? /(^|\.)cdninstagram\.com$|(^|\.)fbcdn\.net$/.test(host)
          : platform === "x"
            ? /(^|\.)twimg\.com$/.test(host)
            : false;
      if (!allowed || !/^https?:$/.test(u.protocol)) return false;
      return /\.(?:mp4|m4v|mov|webm)(?:$|\/|[?#])/i.test(u.pathname + u.search);
    } catch (_) { return false; }
  }

  function onClick(e) {
    e.preventDefault();
    e.stopPropagation();
    if (busy) return;
    busy = true;
    btn.textContent = text().sending;
    const selected = buttonMediaElement && mediaVisible(buttonMediaElement)
      ? { element:buttonMediaElement, type:buttonMediaElement.tagName === "AUDIO" ? "audio" : "video" }
      : biggestMedia();
    const mediaEl = selected?.element;
    const mediaSrc = mediaEl?.currentSrc || mediaEl?.src || "";
    const directAudio = selected?.type === "audio" && /^https?:/i.test(mediaSrc) && /\.(mp3|m4a|aac|ogg|opus|wav|flac)(?:$|[?#])/i.test(mediaSrc);
    const directVideo = selected?.type === "video" && /^https?:/i.test(mediaSrc) && /\.(mp4|webm|m4v|mov)(?:$|[?#])/i.test(mediaSrc);
    const directDocument = /^(video|audio)\//i.test(document.contentType || "");
    const mediaType = selected?.type === "audio" ? "Audio stream" : "Video stream";
    const done = (ok, noPost=false) => {
      btn.textContent = ok ? text().sent : noPost ? text().noPost : text().failed;
      setTimeout(() => { busy=false; if (btn) btn.textContent=lastMediaType==="audio"?AUDIO_ICON:ICON; }, 3500);
    };
    try {
      const platform = socialPlatform();
      const postUrl = platform ? socialPostPermalink(mediaEl) : "";
      const trustedDirect = platform && selected?.type === "video" && isDirectSocialMediaURL(mediaSrc, platform);
      // Prefer the stable post URL. If the feed DOM exposes no permalink but the
      // browser exposes a real platform CDN MP4/WebM URL, send that media URL
      // directly instead of failing immediately. Blob/MSE URLs are not direct
      // downloadable resources and are deliberately not passed to the app.
      const targetUrl = postUrl ||
        (trustedDirect ? mediaSrc :
          (platform ? "" : (directAudio || directVideo ? mediaSrc :
            mediaPageUrl(mediaEl || document.querySelector("video, audio") || document.documentElement))));
      if (!targetUrl) { done(false, true); return; }
      const isDirect = !postUrl && (trustedDirect || directAudio || directVideo || directDocument);
      chrome.runtime.sendMessage({
        type:"video", url:targetUrl, referrer:location.href, direct:isDirect,
        name:(document.title || "").replace(/\s*[-|]\s*(YouTube|Vimeo|Facebook|Instagram|X)$/i,"").trim(),
        mediaType, audioOnly:selected?.type==="audio"
      }, resp => { void chrome.runtime.lastError; done(!!(resp && resp.ok)); });
    } catch (_) { done(false); }
  }

  function ensureButton() {
    if (btn) return btn;
    btn = document.createElement("button");
    btn.type = "button";
    btn.textContent = lastMediaType === "audio" ? AUDIO_ICON : ICON;
    btn.title = "Download video with Sabo DM";
    // A popover lives in the browser's top layer, so it stays visible above a full-screen player.
    if (usePopover) btn.setAttribute("popover", "manual");
    Object.assign(btn.style, {
      position: "fixed", inset: "auto", margin: "0", zIndex: "2147483647",
      padding: "8px 13px", font: "600 14px Segoe UI, Arial, sans-serif", color: "#ffffff",
      background: "#2563eb", border: "2px solid rgba(255,255,255,.9)", borderRadius: "999px",
      cursor: "pointer", boxShadow: "0 2px 10px rgba(0,0,0,.5)", display: "none",
      transition: "opacity .25s", lineHeight: "1",
    });
    btn.addEventListener("mouseenter", () => { if (!busy) btn.textContent = text().media; });
    btn.addEventListener("mouseleave", () => { if (!busy) btn.textContent = lastMediaType === "audio" ? AUDIO_ICON : ICON; });
    btn.addEventListener("click", onClick);
    document.documentElement.appendChild(btn);
    return btn;
  }

  // Without popover support, a full-screen player only draws its own contents,
  // so the button has to live inside it.
  function attach(b) {
    let host = document.documentElement;
    const fs = document.fullscreenElement;
    if (!usePopover && fs && !/^(VIDEO|AUDIO|IMG|IFRAME|CANVAS)$/.test(fs.tagName)) host = fs;
    if (b.parentNode !== host) host.appendChild(b);
  }

  function openPopover(b) {
    if (!b.showPopover) return;
    try {
      if (!b.matches(":popover-open")) b.showPopover();
    } catch (e) {}
  }

  // In full screen the button fades out when the mouse is still, like the player controls.
  function wake() {
    if (!btn) return;
    btn.style.opacity = "1";
    btn.style.pointerEvents = "auto";
    clearTimeout(hideTimer);
    if (document.fullscreenElement) {
      hideTimer = setTimeout(() => {
        if (btn && !busy) {
          btn.style.opacity = "0";
          btn.style.pointerEvents = "none";
        }
      }, 2800);
    }
  }

  function update() {
    const media = biggestMedia();
    if (!media) {
      if (btn) {
        btn.style.display = "none";
        try { if (btn.hidePopover && btn.matches(":popover-open")) btn.hidePopover(); } catch (e) {}
      }
      return;
    }
    const r = media.rect;
    buttonMediaElement = media.element || null;
    lastMediaType = media.type;
    const b = ensureButton();
    if (!busy) b.textContent = media.type === "audio" ? AUDIO_ICON : ICON;
    b.title = media.type === "audio" ? text().audio : text().video;
    attach(b);
    b.style.display = "block";
    openPopover(b);
    const fs = !!document.fullscreenElement;
    // Keep the action visually attached to the detected player. For audio
    // controls use a small offset; never position it relative to a page-wide
    // playback container when a compact control cluster is available.
    b.style.left = "auto";
    const offset = media.type === "audio" ? 6 : 12;
    b.style.right = Math.max(8, window.innerWidth - r.right + offset) + "px";
    b.style.top = Math.max(8, fs ? r.top + Math.min(18, r.height * 0.2) : r.top + 6) + "px";
  }

  document.addEventListener("fullscreenchange", () => {
    if (btn) {
      // hide and show again so the button is stacked above the full-screen element
      try { if (btn.hidePopover && btn.matches(":popover-open")) btn.hidePopover(); } catch (e) {}
    }
    update();
    wake();
  });
  document.addEventListener("mousemove", wake, { passive: true, capture: true });
  window.addEventListener("scroll", update, { passive: true });
  window.addEventListener("resize", update, { passive: true });
  setInterval(update, 1000);
})();
