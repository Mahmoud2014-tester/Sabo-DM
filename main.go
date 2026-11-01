//go:build legacyui

package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	d "github.com/lxn/walk/declarative"
)

func tr(s, lang string) string {
	if lang == "" || lang == "en" {
		return s
	}
	maps := map[string]map[string]string{
		"ar": {"Files  ·  Videos  ·  Torrents": "الملفات · الفيديوهات · التورنت", "No downloads yet": "لا توجد تنزيلات بعد", "Add downloads": "إضافة تنزيلات", "Paste link(s), one per line: files, MediaFire pages, video sites, magnet links or .torrent links": "الصق الروابط (رابط في كل سطر): ملفات، صفحات MediaFire، فيديو، روابط magnet أو .torrent", "⬇  Add to queue": "⬇ إضافة إلى قائمة الانتظار", "▶  Add as video...": "▶ إضافة كفيديو...", "+  Add .torrent file...": "+ إضافة ملف تورنت...", "Save to:": "حفظ في:", "Change...": "تغيير...", "Open output folder": "فتح مجلد التنزيلات", "Name": "الاسم", "Size": "الحجم", "Progress": "التقدم", "Speed": "السرعة", "Status": "الحالة", "Current download": "التنزيل الحالي", "No download selected": "لم يتم تحديد تنزيل", "▶  Start / Resume": "▶ بدء / استئناف", "❚❚  Pause": "❚❚ إيقاف مؤقت", "✖  Remove": "✖ إزالة", "✔  Clear finished": "✔ مسح المكتمل", "Open file": "فتح الملف", "Open folder": "فتح المجلد", "Settings": "الإعدادات", "Downloads at once:": "عدد التنزيلات المتزامنة:", "Max segments per file (1–32):": "الحد الأقصى للمقاطع لكل ملف (1–32):", "     Speed limit (KB/s, 0 = unlimited):": "     حد السرعة (KB/s، 0 = بلا حد):", "     Sign in with:": "     تسجيل الدخول باستخدام:", "Install FFmpeg": "تثبيت FFmpeg", "Update yt-dlp": "تحديث yt-dlp", "Start with Windows (runs quietly in the tray)": "التشغيل مع Windows (في شريط النظام)", "Resume unfinished downloads when the app starts": "استئناف التنزيلات غير المكتملة عند بدء التطبيق", "Ready": "جاهز", "Language:": "اللغة:", "Restart Sabo DM to apply the language change.": "أعد تشغيل Sabo DM لتطبيق تغيير اللغة."},
		"fr": {"Files  ·  Videos  ·  Torrents": "Fichiers · Vidéos · Torrents", "No downloads yet": "Aucun téléchargement", "Add downloads": "Ajouter des téléchargements", "Paste link(s), one per line: files, MediaFire pages, video sites, magnet links or .torrent links": "Collez les liens (un par ligne) : fichiers, pages MediaFire, vidéos, liens magnet ou .torrent", "⬇  Add to queue": "⬇ Ajouter à la file", "▶  Add as video...": "▶ Ajouter comme vidéo...", "+  Add .torrent file...": "+ Ajouter un fichier .torrent...", "Save to:": "Enregistrer dans :", "Change...": "Modifier...", "Open output folder": "Ouvrir le dossier de destination", "Name": "Nom", "Size": "Taille", "Progress": "Progression", "Speed": "Vitesse", "Status": "État", "Current download": "Téléchargement actuel", "No download selected": "Aucun téléchargement sélectionné", "▶  Start / Resume": "▶ Démarrer / Reprendre", "❚❚  Pause": "❚❚ Pause", "✖  Remove": "✖ Supprimer", "✔  Clear finished": "✔ Effacer les terminés", "Open file": "Ouvrir le fichier", "Open folder": "Ouvrir le dossier", "Settings": "Paramètres", "Downloads at once:": "Téléchargements simultanés :", "Max segments per file (1–32):": "Nombre max. de segments par fichier (1–32) :", "     Speed limit (KB/s, 0 = unlimited):": "     Limite de vitesse (KB/s, 0 = illimitée) :", "     Sign in with:": "     Se connecter avec :", "Install FFmpeg": "Installer FFmpeg", "Update yt-dlp": "Mettre à jour yt-dlp", "Start with Windows (runs quietly in the tray)": "Démarrer avec Windows (dans la zone de notification)", "Resume unfinished downloads when the app starts": "Reprendre les téléchargements au démarrage", "Ready": "Prêt", "Language:": "Langue :", "Restart Sabo DM to apply the language change.": "Redémarrez Sabo DM pour appliquer la langue."},
	}
	if m, ok := maps[lang]; ok {
		if v, ok := m[s]; ok {
			return v
		}
	}
	return s
}

// translateCurrent resolves captions translated by a previous language selection back
// to their canonical English key, so switching Arabic <-> French <-> English works repeatedly.
func translateCurrent(s, lang string) string {
	maps := map[string]map[string]string{
		"ar": {"Files  ·  Videos  ·  Torrents": "الملفات · الفيديوهات · التورنت", "No downloads yet": "لا توجد تنزيلات بعد", "Add downloads": "إضافة تنزيلات", "Paste link(s), one per line: files, MediaFire pages, video sites, magnet links or .torrent links": "الصق الروابط (رابط في كل سطر): ملفات، صفحات MediaFire، فيديو، روابط magnet أو .torrent", "⬇  Add to queue": "⬇ إضافة إلى قائمة الانتظار", "▶  Add as video...": "▶ إضافة كفيديو...", "+  Add .torrent file...": "+ إضافة ملف تورنت...", "Save to:": "حفظ في:", "Change...": "تغيير...", "Open output folder": "فتح مجلد التنزيلات", "Name": "الاسم", "Size": "الحجم", "Progress": "التقدم", "Speed": "السرعة", "Status": "الحالة", "Current download": "التنزيل الحالي", "No download selected": "لم يتم تحديد تنزيل", "▶  Start / Resume": "▶ بدء / استئناف", "❚❚  Pause": "❚❚ إيقاف مؤقت", "✖  Remove": "✖ إزالة", "✔  Clear finished": "✔ مسح المكتمل", "Open file": "فتح الملف", "Open folder": "فتح المجلد", "Settings": "الإعدادات", "Downloads at once:": "عدد التنزيلات المتزامنة:", "Max segments per file (1–32):": "الحد الأقصى للمقاطع لكل ملف (1–32):", "     Speed limit (KB/s, 0 = unlimited):": "     حد السرعة (KB/s، 0 = بلا حد):", "     Sign in with:": "     تسجيل الدخول باستخدام:", "Install FFmpeg": "تثبيت FFmpeg", "Update yt-dlp": "تحديث yt-dlp", "Start with Windows (runs quietly in the tray)": "التشغيل مع Windows (في شريط النظام)", "Resume unfinished downloads when the app starts": "استئناف التنزيلات غير المكتملة عند بدء التطبيق", "Ready": "جاهز", "Language:": "اللغة:"},
		"fr": {"Files  ·  Videos  ·  Torrents": "Fichiers · Vidéos · Torrents", "No downloads yet": "Aucun téléchargement", "Add downloads": "Ajouter des téléchargements", "Paste link(s), one per line: files, MediaFire pages, video sites, magnet links or .torrent links": "Collez les liens (un par ligne) : fichiers, pages MediaFire, vidéos, liens magnet ou .torrent", "⬇  Add to queue": "⬇ Ajouter à la file", "▶  Add as video...": "▶ Ajouter comme vidéo...", "+  Add .torrent file...": "+ Ajouter un fichier .torrent...", "Save to:": "Enregistrer dans :", "Change...": "Modifier...", "Open output folder": "Ouvrir le dossier de destination", "Name": "Nom", "Size": "Taille", "Progress": "Progression", "Speed": "Vitesse", "Status": "État", "Current download": "Téléchargement actuel", "No download selected": "Aucun téléchargement sélectionné", "▶  Start / Resume": "▶ Démarrer / Reprendre", "❚❚  Pause": "❚❚ Pause", "✖  Remove": "✖ Supprimer", "✔  Clear finished": "✔ Effacer les terminés", "Open file": "Ouvrir le fichier", "Open folder": "Ouvrir le dossier", "Settings": "Paramètres", "Downloads at once:": "Téléchargements simultanés :", "Max segments per file (1–32):": "Nombre max. de segments par fichier (1–32) :", "     Speed limit (KB/s, 0 = unlimited):": "     Limite de vitesse (KB/s, 0 = illimitée) :", "     Sign in with:": "     Se connecter avec :", "Install FFmpeg": "Installer FFmpeg", "Update yt-dlp": "Mettre à jour yt-dlp", "Start with Windows (runs quietly in the tray)": "Démarrer avec Windows (dans la zone de notification)", "Resume unfinished downloads when the app starts": "Reprendre les téléchargements au démarrage", "Ready": "Prêt", "Language:": "Langue :"},
	}
	for _, translations := range maps {
		for english, translated := range translations {
			if s == translated {
				return tr(english, lang)
			}
		}
	}
	return tr(s, lang)
}

// applyLiveLanguage updates native Windows control captions in-place. It deliberately
// does not recreate the window, interrupt downloads, or require an application restart.
func applyLiveLanguage(mw *walk.MainWindow, tv *walk.TableView, lang string) {
	if mw == nil {
		return
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	getTextLen := user32.NewProc("GetWindowTextLengthW")
	getText := user32.NewProc("GetWindowTextW")
	setText := user32.NewProc("SetWindowTextW")
	enumChildren := user32.NewProc("EnumChildWindows")
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		length, _, _ := getTextLen.Call(hwnd)
		if length == 0 || length > 4096 {
			return 1
		}
		buf := make([]uint16, int(length)+1)
		getText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		current := syscall.UTF16ToString(buf)
		translated := translateCurrent(current, lang)
		if translated != current {
			if ptr, err := syscall.UTF16PtrFromString(translated); err == nil {
				setText.Call(hwnd, uintptr(unsafe.Pointer(ptr)))
			}
		}
		return 1
	})
	enumChildren.Call(uintptr(mw.Handle()), callback, 0)

	// Table headers are drawn by the header control and are not ordinary child captions.
	if tv != nil && tv.Columns() != nil {
		headers := []string{"Name", "Size", "Progress", "Speed", "Status"}
		for i, header := range headers {
			if i < tv.Columns().Len() {
				tv.Columns().At(i).SetTitle(tr(header, lang))
			}
		}
	}
}

//go:embed tray.png
var trayPNG []byte

func defaultDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		dl := filepath.Join(home, "Downloads")
		if st, err := os.Stat(dl); err == nil && st.IsDir() {
			return dl
		}
	}
	wd, _ := os.Getwd()
	return wd
}

func fmtDur(dur time.Duration) string {
	sec := int(dur.Seconds())
	if sec < 0 {
		sec = 0
	}
	h, m, r := sec/3600, (sec%3600)/60, sec%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, r)
	}
	return fmt.Sprintf("%d:%02d", m, r)
}

func appIcon() *walk.Icon {
	if img, err := png.Decode(bytes.NewReader(trayPNG)); err == nil {
		if bmp, err := walk.NewBitmapFromImage(img); err == nil {
			if ic, err := walk.NewIconFromBitmap(bmp); err == nil {
				return ic
			}
		}
	}
	return walk.IconApplication()
}

func speedLimitIndex(kb int) int {
	choices := []int{0, 100, 500, 1000, 5000, 10000}
	for i, v := range choices {
		if kb == v {
			return i
		}
	}
	// Map legacy/custom settings to the nearest available choice.
	best, delta := 0, int(^uint(0)>>1)
	for i, v := range choices {
		d := kb - v
		if d < 0 {
			d = -d
		}
		if d < delta {
			best, delta = i, d
		}
	}
	return best
}

func main() {
	// Started by Windows at login: go straight to the tray.
	background := false
	for _, a := range os.Args[1:] {
		if a == "--background" {
			background = true
		}
	}
	// Only one copy: ask the running one to show itself (unless we were started silently).
	if existingInstance(!background) {
		return
	}

	cfg := loadSettings()
	os.RemoveAll(filepath.Join(configDir(), "tmp")) // leftover temporary login files
	saveDir := cfg.Dir
	if st, err := os.Stat(saveDir); saveDir == "" || err != nil || !st.IsDir() {
		saveDir = defaultDir()
	}
	autoStart := cfg.AutoStart
	if autoStart && !autostartEnabled() { // removed outside the app (for example in Task Manager)
		autoStart = false
	}
	autoAsked := cfg.AutoStartAsked
	autoResume := cfg.AutoResume

	var (
		mw                               *walk.MainWindow
		linksEdit                        *walk.TextEdit
		folderLbl, statusLbl, summaryLbl *walk.Label
		tv                               *walk.TableView
		parEdit, segmentEdit             *walk.NumberEdit
		ni                               *walk.NotifyIcon
		browserCB, langCB, speedCB       *walk.ComboBox
		autoCB, resumeCB                 *walk.CheckBox
		quitting                         bool
		cardName, cardInfo               *walk.Label
		cardBar                          *walk.ProgressBar
		cardUpdate                       func()
	)

	q := &Queue{MaxParallel: cfg.Parallel, MaxSegments: cfg.MaxSegments, Queues: cfg.Queues}
	q.post = func(f func()) { mw.Synchronize(f) }
	q.load()
	setSpeedLimit(int64(cfg.LimitKB) * 1024)
	browserSetting.Store(cfg.CookieBrowser)

	// live summary shown in the header. Keep the text stable while a transfer
	// runs; changing a speed string several times per second makes native
	// Windows layouts repaint unnecessarily and can look like button flicker.
	lastSummaryText := ""
	q.onChange = func() {
		if summaryLbl == nil {
			return
		}
		running, queued, done, failed := 0, 0, 0, 0
		for _, it := range q.Items {
			switch {
			case it.running:
				running++
			case it.Status == stQueued:
				queued++
			case it.Status == stDone:
				done++
			case strings.HasPrefix(it.Status, "Error"):
				failed++
			}
		}
		var parts []string
		if running > 0 {
			parts = append(parts, fmt.Sprintf("%d downloading", running))
		}
		if queued > 0 {
			parts = append(parts, fmt.Sprintf("%d queued", queued))
		}
		if done > 0 {
			parts = append(parts, fmt.Sprintf("%d done", done))
		}
		if failed > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", failed))
		}
		text := strings.Join(parts, "  ·  ")
		if text == "" {
			text = "No downloads yet"
		}
		if text != lastSummaryText {
			summaryLbl.SetText(text)
			lastSummaryText = text
		}
		if cardUpdate != nil {
			cardUpdate()
		}
	}

	setStatus := func(s string) {
		if statusLbl != nil {
			statusLbl.SetText(s)
		}
	}
	persist := func() {
		saveSettings(Settings{
			Dir: saveDir, Parallel: q.MaxParallel, MaxSegments: q.MaxSegments, LimitKB: int(speedLimit.Load() / 1024),
			CookieBrowser: getBrowser(), Language: cfg.Language, AutoStart: autoStart, AutoStartAsked: autoAsked, AutoResume: autoResume,
		})
	}
	selected := func() []*Item {
		var r []*Item
		for _, i := range tv.SelectedIndexes() {
			if i >= 0 && i < len(q.Items) {
				r = append(r, q.Items[i])
			}
		}
		return r
	}
	showWindow := func() {
		mw.Show()
		mw.BringToTop()
	}

	// ---- video helpers ----

	askQuality := func(title, note string, opts []VideoOpt) int {
		labels := make([]string, len(opts))
		for i, o := range opts {
			labels[i] = o.Label
		}
		var dlg *walk.Dialog
		var acceptPB, cancelPB *walk.PushButton
		var cb *walk.ComboBox
		choice := -1
		d.Dialog{
			AssignTo: &dlg, Title: "Choose quality", DefaultButton: &acceptPB, CancelButton: &cancelPB,
			MinSize: d.Size{Width: 440, Height: 170}, Layout: d.VBox{},
			Children: []d.Widget{
				d.Label{Text: title},
				d.Label{Text: note},
				d.ComboBox{AssignTo: &cb, Model: labels, CurrentIndex: 0},
				d.Composite{Layout: d.HBox{MarginsZero: true}, Children: []d.Widget{
					d.HSpacer{},
					d.PushButton{AssignTo: &acceptPB, Text: "Download", OnClicked: func() { choice = cb.CurrentIndex(); dlg.Accept() }},
					d.PushButton{AssignTo: &cancelPB, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
				}},
			},
		}.Run(mw)
		return choice
	}

	installYtDlp := func(then func()) {
		if int(walk.MsgBox(mw, "Sabo DM", "Video downloads need yt-dlp (about 18 MB). Download it now?",
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)) != int(walk.DlgCmdYes) {
			return
		}
		setStatus("Downloading yt-dlp...")
		go func() {
			err := downloadYtDlp(context.Background(), func(done, total int64) {
				if total > 0 {
					mw.Synchronize(func() { setStatus(fmt.Sprintf("Downloading yt-dlp... %d%%", done*100/total)) })
				}
			})
			mw.Synchronize(func() {
				if err != nil {
					setStatus("yt-dlp download failed: " + err.Error())
					return
				}
				setStatus("yt-dlp installed")
				if then != nil {
					then()
				}
			})
		}()
	}

	denoDeclined := false
	installDeno := func(then func()) {
		if int(walk.MsgBox(mw, "Sabo DM", "YouTube now needs a small JavaScript engine (Deno, about 50 MB, downloaded once) for all qualities and for signed-in videos. Download it now?",
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)) != int(walk.DlgCmdYes) {
			denoDeclined = true
			then()
			return
		}
		setStatus("Downloading Deno...")
		go func() {
			err := downloadDeno(context.Background(), func(done, total int64) {
				if total > 0 {
					mw.Synchronize(func() { setStatus(fmt.Sprintf("Downloading Deno... %d%%", done*100/total)) })
				}
			})
			mw.Synchronize(func() {
				if err != nil {
					denoDeclined = true
					setStatus("Deno download failed: " + err.Error())
				} else {
					setStatus("Deno installed")
				}
				then()
			})
		}()
	}

	installFFmpeg := func() {
		if _, verifyErr := ffmpegReady(); verifyErr == nil {
			walk.MsgBox(mw, "Sabo DM", "FFmpeg and FFprobe are installed and passed the execution test.", walk.MsgBoxIconInformation)
			return
		}
		if int(walk.MsgBox(mw, "Install FFmpeg", "Sabo DM will download the Windows FFmpeg Essentials build from gyan.dev over HTTPS and install ffmpeg.exe (and ffprobe.exe when available) into its tools folder. This enables MP3, AAC and other audio conversions. Continue?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)) != int(walk.DlgCmdYes) {
			return
		}
		setStatus("Downloading and installing FFmpeg...")
		go func() {
			err := downloadFFmpeg(context.Background(), func(done, total int64) {
				if total > 0 {
					mw.Synchronize(func() { setStatus(fmt.Sprintf("Downloading FFmpeg... %d%%", done*100/total)) })
				} else {
					mw.Synchronize(func() { setStatus(fmt.Sprintf("Downloading FFmpeg... %.1f MB", float64(done)/(1024*1024))) })
				}
			})
			mw.Synchronize(func() {
				if err != nil {
					setStatus("FFmpeg installation failed: " + err.Error())
					walk.MsgBox(mw, "Sabo DM", "Could not install FFmpeg:\n\n"+err.Error(), walk.MsgBoxIconWarning)
					return
				}
				if _, verifyErr := ffmpegReady(); verifyErr != nil {
					setStatus("FFmpeg verification failed: " + verifyErr.Error())
					walk.MsgBox(mw, "Sabo DM", "FFmpeg was downloaded but is not usable:\n\n"+verifyErr.Error(), walk.MsgBoxIconWarning)
					return
				}
				setStatus("FFmpeg installed and verified — audio conversion is ready")
				walk.MsgBox(mw, "Sabo DM", "FFmpeg and FFprobe were installed and both passed the execution test. Audio conversion is ready.", walk.MsgBoxIconInformation)
			})
		}()
	}

	var addVideos func(links []string, jar, referer string, audioOnly bool)
	addVideos = func(links []string, jar, referer string, audioOnly bool) {
		if findTool("yt-dlp.exe") == "" {
			installYtDlp(func() { addVideos(links, jar, referer, audioOnly) })
			return
		}
		if !denoDeclined && findTool("deno.exe") == "" && anyYouTube(links) {
			installDeno(func() { addVideos(links, jar, referer, audioOnly) })
			return
		}
		go func() {
			_, ffErr := ffmpegReady()
			noFF := ffErr != nil
			for _, l := range links {
				l := l
				if len(links) > 1 { // several links: take "Best available" for each
					mw.Synchronize(func() { q.AddVideo(l, saveDir, "", "Best available", bestArgs(!noFF), jar) })
					continue
				}
				mw.Synchronize(func() { setStatus("Reading video info...") })
				info, err := fetchInfo(context.Background(), l, jar, referer)
				if err != nil {
					msg := loginHint(err.Error())
					mw.Synchronize(func() {
						setStatus("Video error: " + msg)
						walk.MsgBox(mw, "Sabo DM", "Could not read this video:\n\n"+msg, walk.MsgBoxIconWarning)
					})
					continue
				}
				opts := videoOptions(info, !noFF)
				if audioOnly {
					opts = audioOptions(!noFF)
				}
				note := ""
				if noFF {
					note = "FFmpeg/FFprobe are not installed together. Install both to enable MP3 conversion and MP4 video output. Without them, video downloads keep the source format (which may be WebM)."
				}
				choice := -1
				done := make(chan struct{})
				mw.Synchronize(func() {
					setStatus("Ready")
					dialogTitle := "Choose quality"
					if audioOnly {
						dialogTitle = "Choose audio format"
					}
					choice = askQuality(dialogTitle, note, opts)
					close(done)
				})
				<-done
				if choice < 0 {
					continue
				}
				o := opts[choice]
				mw.Synchronize(func() { q.AddVideo(l, saveDir, info.Title, o.Label, o.Args, jar) })
			}
		}()
	}

	var addTorrents func(srcs []string)
	installAria2 := func(then func()) {
		if int(walk.MsgBox(mw, "Sabo DM", "Torrent downloads need aria2 (about 6 MB, downloaded once from GitHub and checked for safety). Download it now?",
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)) != int(walk.DlgCmdYes) {
			return
		}
		setStatus("Downloading aria2...")
		go func() {
			err := downloadAria2(context.Background(), func(done, total int64) {
				if total > 0 {
					mw.Synchronize(func() { setStatus(fmt.Sprintf("Downloading aria2... %d%%", done*100/total)) })
				}
			})
			mw.Synchronize(func() {
				if err != nil {
					setStatus("aria2 download failed: " + err.Error())
					return
				}
				setStatus("aria2 installed")
				if then != nil {
					then()
				}
			})
		}()
	}
	addTorrents = func(srcs []string) {
		if findTool("aria2c.exe") == "" {
			installAria2(func() { addTorrents(srcs) })
			return
		}
		go func() {
			for _, src := range srcs {
				mw.Synchronize(func() { setStatus("Reading torrent info (magnet links can take a minute)...") })
				meta, err := prepareTorrent(context.Background(), src)
				if err != nil {
					msg := loginHint(err.Error())
					mw.Synchronize(func() {
						setStatus("Torrent error: " + msg)
						walk.MsgBox(mw, "Sabo DM", "Could not read this torrent:\n\n"+msg, walk.MsgBoxIconWarning)
					})
					continue
				}
				sel, ok := "", true
				if meta.visible() > 1 { // several files: let the user choose
					done := make(chan struct{})
					mw.Synchronize(func() {
						setStatus("Ready")
						sel, ok = askFiles(mw, meta)
						close(done)
					})
					<-done
				}
				if !ok {
					continue
				}
				mw.Synchronize(func() {
					setStatus("Ready")
					q.AddTorrent(meta.Path, saveDir, meta.Name, sel)
				})
			}
		}()
	}
	addTorrentFile := func() {
		dlg := new(walk.FileDialog)
		dlg.Title = "Choose a .torrent file"
		dlg.Filter = "Torrent files (*.torrent)|*.torrent|All files (*.*)|*.*"
		if ok, _ := dlg.ShowOpen(mw); ok {
			addTorrents([]string{dlg.FilePath})
		}
	}

	updateYt := func() {
		if findTool("yt-dlp.exe") == "" {
			installYtDlp(nil)
			return
		}
		setStatus("Updating yt-dlp...")
		go func() {
			out, err := updateYtDlp()
			mw.Synchronize(func() {
				if err != nil {
					setStatus("Update failed: " + err.Error())
					return
				}
				setStatus(out)
			})
		}()
	}

	// ---- actions ----

	addAll := func(forceVideo bool) {
		lines := strings.FieldsFunc(linksEdit.Text(), func(r rune) bool { return r == '\n' || r == '\r' })
		var files, videos, torrents []string
		bad := 0
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if !strings.HasPrefix(l, "http://") && !strings.HasPrefix(l, "https://") &&
				!strings.HasPrefix(strings.ToLower(l), "magnet:") {
				bad++
				continue
			}
			switch {
			case isTorrentLink(l):
				torrents = append(torrents, l)
			case forceVideo || isVideoSite(l):
				videos = append(videos, l)
			default:
				files = append(files, l)
			}
		}
		if bad > 0 {
			walk.MsgBox(mw, "Sabo DM", fmt.Sprintf("%d line(s) were skipped because they do not start with http://, https:// or magnet:", bad), walk.MsgBoxIconWarning)
		}
		for _, l := range files {
			q.Add(l, saveDir)
		}
		if len(videos) > 0 {
			addVideos(videos, "", "", false)
		}
		if len(torrents) > 0 {
			addTorrents(torrents)
		}
		if len(files)+len(videos)+len(torrents) > 0 {
			linksEdit.SetText("")
		}
	}

	chooseFolder := func() {
		dlg := new(walk.FileDialog)
		dlg.Title = "Choose download folder"
		if ok, _ := dlg.ShowBrowseFolder(mw); ok {
			saveDir = dlg.FilePath
			folderLbl.SetText(saveDir)
			persist()
		}
	}

	// Always opens the folder where downloads are saved.
	openOutput := func() {
		if err := openFolder(saveDir); err != nil {
			walk.MsgBox(mw, "Sabo DM", "Could not open the folder:\n\n"+err.Error(), walk.MsgBoxIconWarning)
		}
	}
	// With a row selected: the folder of that download (file highlighted).
	// With nothing selected: the output folder.
	openFolderBtn := func() {
		sel := selected()
		if len(sel) == 0 {
			openOutput()
			return
		}
		it := sel[0]
		var err error
		if it.Path != "" {
			err = revealPath(it.Path)
		} else {
			err = openFolder(it.Dir)
		}
		if err != nil {
			walk.MsgBox(mw, "Sabo DM", "Could not open the folder:\n\n"+err.Error(), walk.MsgBoxIconWarning)
		}
	}
	openSel := func() {
		for _, it := range selected() {
			if it.Status != stDone || it.Path == "" {
				continue
			}
			if _, err := os.Stat(it.Path); err != nil {
				walk.MsgBox(mw, "Sabo DM", "This file is no longer there:\n\n"+it.Path, walk.MsgBoxIconWarning)
				continue
			}
			openFile(it.Path)
		}
	}

	// ---- window ----

	browserNames := []string{"", "firefox", "chrome", "edge", "brave"}
	browserIdx := 0
	for i, n := range browserNames {
		if n == cfg.CookieBrowser {
			browserIdx = i
		}
	}

	white := walk.RGB(255, 255, 255)
	slate := walk.RGB(71, 85, 105)
	bold := d.Font{Family: "Segoe UI Semibold", PointSize: 10, Bold: true}
	btnSize := d.Size{Width: 190, Height: 40}

	err := d.MainWindow{
		AssignTo:   &mw,
		Title:      "Sabo DM",
		MinSize:    d.Size{Width: 900, Height: 600},
		Font:       d.Font{Family: "Segoe UI", PointSize: 10},
		Background: d.SolidColorBrush{Color: walk.RGB(248, 250, 252)},
		Visible:    !background,
		Layout:     d.VBox{MarginsZero: true, Spacing: 0},
		Children: []d.Widget{
			// ---- header band ----
			d.Composite{
				Background: d.SolidColorBrush{Color: walk.RGB(17, 24, 39)},
				Layout:     d.HBox{Margins: d.Margins{Left: 24, Top: 16, Right: 24, Bottom: 16}, Spacing: 18},
				Children: []d.Widget{
					d.Label{Text: "Sabo DM", TextColor: white, Font: d.Font{Family: "Segoe UI Semibold", PointSize: 23}},
					d.Label{Text: tr("Files  ·  Videos  ·  Torrents", cfg.Language), TextColor: walk.RGB(191, 205, 224), Font: d.Font{Family: "Segoe UI", PointSize: 10}},
					d.HSpacer{},
					d.Label{AssignTo: &summaryLbl, Text: tr("No downloads yet", cfg.Language), TextColor: walk.RGB(226, 232, 240), Font: d.Font{Family: "Segoe UI Semibold", PointSize: 10},
						MinSize: d.Size{Width: 380}, TextAlignment: d.AlignFar},
				},
			},
			// ---- page ----
			d.Composite{
				Layout: d.VBox{Margins: d.Margins{Left: 10, Top: 8, Right: 10, Bottom: 6}, Spacing: 6},
				Children: []d.Widget{
					d.GroupBox{
						Title:  tr("Add downloads", cfg.Language),
						Layout: d.VBox{Margins: d.Margins{Left: 10, Top: 8, Right: 10, Bottom: 10}, Spacing: 8},
						Children: []d.Widget{
							d.Label{Text: tr("Paste link(s), one per line: files, MediaFire pages, video sites, magnet links or .torrent links", cfg.Language), TextColor: slate},
							d.TextEdit{AssignTo: &linksEdit, VScroll: true, MinSize: d.Size{Height: 48}, MaxSize: d.Size{Height: 78}},
							d.Composite{
								Layout: d.HBox{MarginsZero: true, Spacing: 8},
								Children: []d.Widget{
									d.PushButton{Text: tr("⬇  Add to queue", cfg.Language), MinSize: btnSize, OnClicked: func() { addAll(false) }},
									d.PushButton{Text: tr("▶  Add as video...", cfg.Language), MinSize: btnSize, OnClicked: func() { addAll(true) }},
									d.PushButton{Text: tr("+  Add .torrent file...", cfg.Language), MinSize: d.Size{Width: 210, Height: 38}, OnClicked: addTorrentFile},
									d.HSpacer{},
								},
							},
							d.Composite{
								Layout: d.HBox{MarginsZero: true, Spacing: 8},
								Children: []d.Widget{
									d.Label{Text: tr("Save to:", cfg.Language), Font: bold},
									d.Label{AssignTo: &folderLbl, Text: saveDir, TextColor: walk.RGB(37, 99, 235)},
									d.HSpacer{},
									d.PushButton{Text: tr("Change...", cfg.Language), OnClicked: chooseFolder},
									d.PushButton{Text: tr("Open output folder", cfg.Language), OnClicked: openOutput},
								},
							},
						},
					},
					d.TableView{
						AssignTo:         &tv,
						AlternatingRowBG: true,
						MinSize:          d.Size{Height: 110},
						MultiSelection:   true,
						OnItemActivated:  openSel,
						OnSelectedIndexesChanged: func() {
							if cardUpdate != nil {
								cardUpdate()
							}
						},
						Columns: []d.TableViewColumn{
							{Title: tr("Name", cfg.Language), Width: 300},
							{Title: tr("Size", cfg.Language), Width: 85},
							{Title: tr("Progress", cfg.Language), Width: 185},
							{Title: tr("Speed", cfg.Language), Width: 95},
							{Title: tr("Status", cfg.Language), Width: 240},
						},
						StyleCell: func(style *walk.CellStyle) {
							if c := style.Col(); c < 2 || c > 4 {
								return
							}
							row := style.Row()
							if row < 0 || row >= len(q.Items) {
								return
							}
							it := q.Items[row]
							switch {
							case it.Status == stDone:
								style.TextColor = walk.RGB(22, 163, 74) // green: completed
							case it.running:
								style.TextColor = walk.RGB(220, 38, 38) // red: downloading
							case strings.HasPrefix(it.Status, "Error"):
								style.TextColor = walk.RGB(234, 88, 12)
							default:
								style.TextColor = walk.RGB(107, 114, 128)
							}
						},
						Model: q,
					},
					d.GroupBox{
						Title:  tr("Current download", cfg.Language),
						Layout: d.VBox{Margins: d.Margins{Left: 10, Top: 6, Right: 10, Bottom: 8}, Spacing: 4},
						Children: []d.Widget{
							d.Label{AssignTo: &cardName, Text: tr("No download selected", cfg.Language), Font: bold},
							d.ProgressBar{AssignTo: &cardBar, MinValue: 0, MaxValue: 1000, MinSize: d.Size{Height: 20}},
							d.Label{AssignTo: &cardInfo, Text: " ", TextColor: slate},
						},
					},
					d.Composite{
						Layout: d.HBox{MarginsZero: true, Spacing: 8},
						Children: []d.Widget{
							d.PushButton{Text: tr("▶  Start / Resume", cfg.Language), MinSize: btnSize, OnClicked: func() {
								for _, it := range selected() {
									q.Start(it)
								}
							}},
							d.PushButton{Text: tr("❚❚  Pause", cfg.Language), MinSize: d.Size{Width: 110, Height: 32}, OnClicked: func() {
								for _, it := range selected() {
									q.Pause(it)
								}
							}},
							d.PushButton{Text: tr("✖  Remove", cfg.Language), MinSize: d.Size{Width: 110, Height: 32}, OnClicked: func() { q.Remove(tv.SelectedIndexes()) }},
							d.PushButton{Text: tr("✔  Clear finished", cfg.Language), MinSize: btnSize, OnClicked: func() { q.ClearDone() }},
							d.HSpacer{},
							d.PushButton{Text: tr("Open file", cfg.Language), MinSize: d.Size{Width: 100, Height: 32}, OnClicked: openSel},
							d.PushButton{Text: tr("Open folder", cfg.Language), MinSize: d.Size{Width: 110, Height: 32}, OnClicked: openFolderBtn},
						},
					},
					d.GroupBox{
						Title:  tr("Settings", cfg.Language),
						Layout: d.VBox{Margins: d.Margins{Left: 10, Top: 8, Right: 10, Bottom: 8}, Spacing: 6},
						Children: []d.Widget{
							d.Composite{
								Layout: d.HBox{MarginsZero: true, Spacing: 8},
								Children: []d.Widget{
									d.Label{Text: tr("Downloads at once:", cfg.Language)},
									d.NumberEdit{
										AssignTo: &parEdit, Value: float64(cfg.Parallel), MinValue: 1, MaxValue: 8, Decimals: 0,
										MaxSize: d.Size{Width: 50},
										OnValueChanged: func() {
											if parEdit == nil {
												return
											}
											q.MaxParallel = int(parEdit.Value())
											persist()
											q.schedule()
										},
									},
									d.Label{Text: tr("     Max segments per file (1–32):", cfg.Language)},
									d.NumberEdit{
										AssignTo: &segmentEdit, Value: float64(cfg.MaxSegments), MinValue: 1, MaxValue: 32, Decimals: 0,
										MaxSize: d.Size{Width: 48},
										OnValueChanged: func() {
											if segmentEdit != nil {
												q.MaxSegments = int(segmentEdit.Value())
												persist()
											}
										},
									},
									d.Label{Text: tr("     Speed limit:", cfg.Language)},
									d.ComboBox{
										AssignTo:     &speedCB,
										Model:        []string{"Unlimited (0 KB/s)", "100 KB/s", "500 KB/s", "1,000 KB/s (1 MB/s)", "5,000 KB/s (5 MB/s)", "10,000 KB/s (10 MB/s)"},
										CurrentIndex: speedLimitIndex(cfg.LimitKB), MinSize: d.Size{Width: 245}, MaxSize: d.Size{Width: 270},
										OnCurrentIndexChanged: func() {
											if speedCB == nil {
												return
											}
											choices := []int{0, 100, 500, 1000, 5000, 10000}
											i := speedCB.CurrentIndex()
											if i >= 0 && i < len(choices) {
												setSpeedLimit(int64(choices[i]) * 1024)
												persist()
											}
										},
									},
									d.Label{Text: tr("     Sign in with:", cfg.Language)},
									d.ComboBox{
										AssignTo: &browserCB, Model: []string{"None", "Firefox", "Chrome", "Edge", "Brave"}, CurrentIndex: browserIdx,
										MinSize: d.Size{Width: 100}, MaxSize: d.Size{Width: 110},
										OnCurrentIndexChanged: func() {
											if browserCB == nil {
												return
											}
											if i := browserCB.CurrentIndex(); i >= 0 && i < len(browserNames) {
												browserSetting.Store(browserNames[i])
												persist()
											}
										},
									},
									d.HSpacer{},
									d.Label{Text: tr("Language:", cfg.Language)},
									d.ComboBox{AssignTo: &langCB, Model: []string{"English", "العربية", "Français"}, CurrentIndex: map[string]int{"en": 0, "ar": 1, "fr": 2}[cfg.Language], MinSize: d.Size{Width: 120}, MaxSize: d.Size{Width: 130}, OnCurrentIndexChanged: func() {
										if langCB == nil {
											return
										}
										langs := []string{"en", "ar", "fr"}
										i := langCB.CurrentIndex()
										if i >= 0 && i < len(langs) && cfg.Language != langs[i] {
											cfg.Language = langs[i]
											setAppLanguage(cfg.Language)
											persist()
											applyLiveLanguage(mw, tv, cfg.Language)
											if statusLbl != nil {
												statusLbl.SetText(tr("Ready", cfg.Language))
											}
										}
									}},
									d.PushButton{Text: tr("Install FFmpeg", cfg.Language), OnClicked: installFFmpeg},
									d.PushButton{Text: tr("Update yt-dlp", cfg.Language), OnClicked: updateYt},
								},
							},
							d.Composite{
								Layout: d.HBox{MarginsZero: true, Spacing: 24},
								Children: []d.Widget{
									d.CheckBox{
										AssignTo: &autoCB, Text: tr("Start with Windows (runs quietly in the tray)", cfg.Language), Checked: autoStart,
										OnCheckedChanged: func() {
											if autoCB == nil {
												return
											}
											on := autoCB.Checked()
											if on == autoStart {
												return
											}
											autoStart = on
											if err := setAutostart(on); err != nil {
												setStatus("Could not change Windows startup: " + err.Error())
											}
											persist()
										},
									},
									d.CheckBox{
										AssignTo: &resumeCB, Text: tr("Resume unfinished downloads when the app starts", cfg.Language), Checked: autoResume,
										OnCheckedChanged: func() {
											if resumeCB == nil {
												return
											}
											autoResume = resumeCB.Checked()
											persist()
										},
									},
									d.HSpacer{},
								},
							},
						},
					},
					d.Composite{MinSize: d.Size{Height: 34}, Layout: d.HBox{Margins: d.Margins{Left: 4, Top: 5, Right: 4, Bottom: 5}, Spacing: 0}, Children: []d.Widget{d.Label{AssignTo: &statusLbl, Text: tr("Ready", cfg.Language), TextColor: slate, MinSize: d.Size{Height: 24}}, d.HSpacer{}}},
				},
			},
		},
	}.Create()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Restore the native Windows title bar and standard window controls.
	ensureStandardWindowFrame(uintptr(mw.Handle()))
	icon := appIcon()
	mw.SetIcon(icon)

	// ---- the "current download" card: red bar while downloading, green when completed ----

	initBar(uintptr(cardBar.Handle()))
	lastBarColor := uint32(0)
	lastCardName, lastCardInfo := "", ""
	lastCardValue := -1
	cardUpdate = func() {
		if cardName == nil || cardBar == nil || cardInfo == nil || tv == nil {
			return
		}
		var it *Item
		if sel := selected(); len(sel) > 0 {
			it = sel[0]
		} else {
			for _, x := range q.Items {
				if x.running {
					it = x
					break
				}
			}
			if it == nil && len(q.Items) > 0 {
				it = q.Items[len(q.Items)-1]
			}
		}
		if it == nil {
			if lastCardName != "No download selected" {
				cardName.SetText("No download selected")
				lastCardName = "No download selected"
			}
			if lastCardInfo != " " {
				cardInfo.SetText(" ")
				lastCardInfo = " "
			}
			if lastCardValue != 0 {
				cardBar.SetValue(0)
				lastCardValue = 0
			}
			return
		}
		pct := 0.0
		switch {
		case it.Status == stDone:
			pct = 100
		case it.Total > 0:
			pct = float64(it.Done) * 100 / float64(it.Total)
		}
		col := colGray
		switch {
		case it.Status == stDone:
			col = colGreen
		case it.running:
			col = colRed
		case strings.HasPrefix(it.Status, "Error"):
			col = colOrange
		}
		if col != lastBarColor {
			setBarColor(uintptr(cardBar.Handle()), col)
			lastBarColor = col
		}
		info := fmt.Sprintf("%.1f%%", pct)
		if it.Total > 0 {
			info += fmt.Sprintf("     %s of %s", humanBytes(float64(it.Done)), humanBytes(float64(it.Total)))
		}
		if it.running && it.Speed > 0 {
			info += "     " + humanBytes(it.Speed) + "/s"
			if it.Total > it.Done {
				info += "     ETA " + fmtDur(time.Duration(float64(it.Total-it.Done)/it.Speed*float64(time.Second)))
			}
		}
		info += "     " + it.Status
		name := displayName(it)
		if name != lastCardName {
			cardName.SetText(name)
			lastCardName = name
		}
		if info != lastCardInfo {
			cardInfo.SetText(info)
			lastCardInfo = info
		}
		value := int(pct * 10)
		if value != lastCardValue {
			cardBar.SetValue(value)
			lastCardValue = value
		}
	}
	cardUpdate()

	// ---- tray icon: the close button hides the window, downloads keep running ----

	if n, nerr := walk.NewNotifyIcon(mw); nerr == nil {
		ni = n
		defer ni.Dispose()
		ni.SetIcon(icon)
		ni.SetToolTip("Sabo DM")
		ni.SetVisible(true)
		ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
			if button == walk.LeftButton {
				showWindow()
			}
		})
		showAct := walk.NewAction()
		showAct.SetText("Show Sabo DM")
		showAct.Triggered().Attach(showWindow)
		folderAct := walk.NewAction()
		folderAct.SetText("Open output folder")
		folderAct.Triggered().Attach(openOutput)
		exitAct := walk.NewAction()
		exitAct.SetText("Exit")
		exitAct.Triggered().Attach(func() {
			quitting = true
			q.CancelAll()
			mw.Close()
		})
		ni.ContextMenu().Actions().Add(showAct)
		ni.ContextMenu().Actions().Add(folderAct)
		ni.ContextMenu().Actions().Add(exitAct)
	}
	q.onDone = func(it *Item) {
		if ni != nil {
			name := it.Name
			if name == "" {
				name = it.URL
			}
			ni.ShowInfo("Download complete", name)
		}
	}
	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if quitting || ni == nil {
			q.CancelAll()
			return
		}
		*canceled = true
		mw.Hide()
		ni.ShowInfo("Sabo DM is still running", "Downloads continue in the background. Use the tray icon to open or exit.")
	})

	// ---- browser extension API ----

	setAppLanguage(cfg.Language)
	startDashboardServer(q, saveDir, func(f func()) { mw.Synchronize(f) }, func() { mw.Synchronize(func() { quitting = true; mw.Close() }) })
	startServer(func(r addReq) error {
		mw.Synchronize(func() {
			switch r.Kind {
			case "video", "audio":
				showWindow()
				if r.Action == "queue" {
					noFF := findFFmpegDir() == ""
					q.AddVideoQueuedWithReferrer(r.URL, saveDir, r.Name, "Best available", bestArgs(!noFF), netscapeJar(r.Cookies), r.Referrer)
				} else {
					addVideos([]string{r.URL}, netscapeJar(r.Cookies), r.Referrer, r.Kind == "audio")
				}
			case "torrent":
				if r.Action == "queue" {
					q.AddTorrentQueued(r.URL, saveDir, r.Name, "")
				} else {
					addTorrents([]string{r.URL})
				}
			default:
				if r.Action == "queue" {
					q.AddExtQueuedNamed(r.URL, saveDir, r.Referrer, r.Cookie, r.Name)
				} else {
					showWindow()
					q.AddExtNamed(r.URL, saveDir, r.Referrer, r.Cookie, r.Name)
				}
			}
		})
		return nil
	}, func() { mw.Synchronize(showWindow) }, func(url string) map[string]interface{} {
		ch := make(chan map[string]interface{}, 1)
		mw.Synchronize(func() {
			for i := len(q.Items) - 1; i >= 0; i-- {
				it := q.Items[i]
				if it.URL == url {
					pct := 0.0
					if it.Total > 0 {
						pct = float64(it.Done) * 100 / float64(it.Total)
					}
					ch <- map[string]interface{}{"name": displayName(it), "status": it.Status, "done": it.Done, "total": it.Total, "speed": it.Speed, "percent": pct}
					return
				}
			}
			ch <- map[string]interface{}{"name": "Download", "status": "Queued", "done": int64(0), "total": int64(0), "speed": float64(0), "percent": float64(0)}
		})
		select {
		case result := <-ch:
			return result
		case <-time.After(2 * time.Second):
			return map[string]interface{}{"status": "Checking", "percent": float64(0)}
		}
	})

	// ---- start-up behaviour ----

	if autoStart { // keep the Windows startup entry pointing at this copy of the app
		go setAutostart(true)
	}
	if background {
		if ni != nil {
			mw.Synchronize(func() { mw.Hide() }) // started by Windows: tray only
		} else {
			mw.Synchronize(showWindow) // no tray available: never leave the app invisible
		}
	} else if !autoAsked {
		mw.Synchronize(func() {
			yes := int(walk.MsgBox(mw, "Sabo DM",
				"Start Sabo DM automatically when Windows starts?\n\nIt runs quietly in the system tray so it can catch downloads from your browser. You can change this any time under Settings.",
				walk.MsgBoxYesNo|walk.MsgBoxIconQuestion)) == int(walk.DlgCmdYes)
			autoAsked = true
			if autoCB != nil {
				autoCB.SetChecked(yes) // the check box handler applies and saves it
			}
			persist()
		})
	}
	if autoResume {
		delay := time.Second
		if background {
			delay = 20 * time.Second // give the network time to come up after login
		}
		time.AfterFunc(delay, func() {
			mw.Synchronize(func() {
				for _, it := range q.Items {
					if it.Status == stPaused {
						q.Start(it)
					}
				}
			})
		})
	}
	q.notify()

	mw.Run()
}
