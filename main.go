package main

// PotBS chat

import (
	"log"

	"os"

	"regexp"
	str "strings"

	"path/filepath"
	"strconv"

	"errors"

	"runtime"
	"time"

	gotr "github.com/bas24/googletranslatefree"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/hpcloud/tail"
	libretr "github.com/snakesel/libretranslate"
	"gopkg.in/ini.v1"
)

var (
	cfg   *ini.File
	libre *libretr.Translation
)

// IDs to access the tree view columns by
const (
	COLUMN_DATE = iota
	COLUMN_TEXT
)

const (
	transToRU = "RU"
	transToEN = "EN"
)

const (
	title   = "PotBS chat message"
	cfgFile = "./potbs-chat.ini"
)

type replacement struct {
	text   string
	start  int
	lenght int
}

type MainWindow struct {
	Window *gtk.Window

	TreeView  *gtk.TreeView
	ListStore *gtk.ListStore

	LineSelection *gtk.TreeSelection

	BtnClear *gtk.Button
	BtnNew   *gtk.Button
	BtnExit  *gtk.Button

	ComboTransTo *gtk.ComboBoxText
	LblTransTo   *gtk.Label

	tailQuit bool

	pathToLog string

	SpinBtn *gtk.SpinButton // Задержка отправки сообщения (сек)

	darkToggle *gtk.ToggleButton
}

func main() {
	var err error

	// Загрузка настроек
	cfg, err = ini.LooseLoad(cfgFile)
	if err != nil {
		log.Fatalf("[ERR]\tFail to read file: %v", err)
	}

	potbs_logdir := "./"

	// Если передан параметр, бурем путь из него,
	// иначе из файла настроек
	if len(os.Args) > 1 {
		potbs_logdir = filepath.Clean(os.Args[1])
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			switch runtime.GOOS {
			case "windows":
				potbs_logdir = cfg.Section("Main").Key("LogDir").MustString(filepath.Join(home, "Documents/Pirates of the Burning Sea/log"))
			default:
				potbs_logdir = cfg.Section("Main").Key("LogDir").MustString(filepath.Join(home, "Pirates of the Burning Sea/log"))
			}
		}
	}

	// Initialize GTK without parsing any command line arguments.
	gtk.Init(nil)

	mainUI := mainWindowCreate()

	// ### применяем настроки
	mainUI.Window.Resize(cfg.Section("Main").Key("width").MustInt(800), cfg.Section("Main").Key("height").MustInt(600))
	mainUI.Window.Move(cfg.Section("Main").Key("posX").MustInt(0), cfg.Section("Main").Key("posY").MustInt(0))
	mainUI.ComboTransTo.SetActiveID(cfg.Section("Main").Key("transTo").MustString(transToRU))
	mainUI.SpinBtn.SetValue(cfg.Section("Main").Key("delaySending").MustFloat64(3))
	mainUI.darkToggle.SetActive(cfg.Section("Main").Key("dark").MustBool(false))

	// Recursively show all widgets contained in this window.
	mainUI.Window.ShowAll()

	//mainUI.Window.SetPosition(gtk.WIN_POS_CENTER)
	mainUI.pathToLog = potbs_logdir
	loadAndRun(mainUI)

	//gtk.SetInteractiveDebugging(true)

	libre = libretr.New(libretr.Config{Key: "potbs-chat"})
	gtk.Main()

}

func mainWindowCreate() *MainWindow {
	var err error

	win := new(MainWindow)

	// Create a new toplevel window, set its title, and connect it to the
	// "destroy" signal to exit the GTK main loop when it is destroyed.
	win.Window, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	checkErr(err, "Unable to create window")

	win.Window.SetTitle(title)

	win.Window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	win.Window.Connect("delete-event", func() {
		cfg.Section("Main").Key("LogDir").SetValue(win.pathToLog)
		cfg.Section("Main").Key("transTo").SetValue(win.ComboTransTo.GetActiveID())
		cfg.Section("Main").Key("delaySending").SetValue(strconv.Itoa(win.SpinBtn.GetValueAsInt()))
		cfg.Section("Main").Key("dark").SetValue(strconv.FormatBool(win.darkToggle.GetActive()))

		w, h := win.Window.GetSize()
		cfg.Section("Main").Key("width").SetValue(strconv.Itoa(w))
		cfg.Section("Main").Key("height").SetValue(strconv.Itoa(h))

		x, y := win.Window.GetPosition()
		cfg.Section("Main").Key("posX").SetValue(strconv.Itoa(x))
		cfg.Section("Main").Key("posY").SetValue(strconv.Itoa(y))

		cfg.SaveTo(cfgFile)

	})

	win.tailQuit = false

	// Получаем остальные объекты MainWindow
	win.TreeView, err = gtk.TreeViewNew()
	checkErr(err)
	win.TreeView.SetHExpand(true)
	win.TreeView.SetFixedHeightMode(false) // режим фиксированной одинаковой высоты строк
	win.TreeView.AppendColumn(createTextColumn("Time", COLUMN_DATE))
	//win.TreeView.AppendColumn(createTextColumn("Text", COLUMN_TEXT))

	cellRendererTEXT, err := gtk.CellRendererTextNew()
	checkErr(err, "Unable to create text cell renderer")
	cellRendererTEXT.SetProperty("wrap-mode", gtk.WRAP_WORD)

	columnTEXT, err := gtk.TreeViewColumnNewWithAttribute("Text", cellRendererTEXT, "text", COLUMN_TEXT)
	checkErr(err, "Unable to create cell column")

	win.TreeView.AppendColumn(columnTEXT)
	columnTEXT.SetSizing(gtk.TREE_VIEW_COLUMN_AUTOSIZE)
	columnTEXT.SetExpand(true)
	columnTEXT.SetResizable(true)
	//columnTEXT.SetMinWidth(100)

	// Перенос текста по ширине столбца
	//win.Window.ConnectAfter("check-resize", func() {
	columnTEXT.ConnectAfter("notify::width", func() {
		cellRendererTEXT.SetProperty("wrap-width", columnTEXT.GetWidth())
		//log.Printf("change width col. to : %d", columnTEXT.GetWidth())
	})

	win.ListStore, err = gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_STRING)
	checkErr(err)

	win.TreeView.SetModel(win.ListStore)

	win.LineSelection, err = win.TreeView.GetSelection()
	checkErr(err)
	win.LineSelection.SetMode(gtk.SELECTION_SINGLE)

	win.BtnClear, err = gtk.ButtonNew()
	checkErr(err)
	win.BtnClear.SetLabel("Clear")

	win.BtnClear.Connect("clicked", func() {
		win.ListStore.Clear()
	})

	win.BtnExit, err = gtk.ButtonNew()
	checkErr(err)
	win.BtnExit.SetLabel("Exit")

	win.BtnExit.Connect("clicked", func() {
		win.Window.Close()
	})

	win.BtnNew, err = gtk.ButtonNew()
	checkErr(err)
	win.BtnNew.SetLabel("Reload")

	win.BtnNew.Connect("clicked", func() {
		win.tailQuit = true
		win.ListStore.Clear()

		loadAndRun(win)
	})

	win.SpinBtn, err = gtk.SpinButtonNewWithRange(0, 5, 1)
	checkErr(err)
	win.SpinBtn.SetDigits(0)

	// win.SpinBtn.Connect("value-changed", func() {
	// 	log.Printf("Delay sending set to: %.0f", win.SpinBtn.GetValue())
	// })

	scroll, err := gtk.ScrolledWindowNew(nil, nil)
	scroll.Add(win.TreeView)
	scroll.SetVExpand(true) //расширяемость по вертикали

	// Авто скрол
	win.TreeView.Connect("size-allocate", func() {
		adj := scroll.GetVAdjustment()
		adj.SetValue(adj.GetUpper() - adj.GetPageSize())
	})

	win.ComboTransTo, err = gtk.ComboBoxTextNew()
	checkErr(err)
	win.ComboTransTo.Append(transToRU, transToRU)
	win.ComboTransTo.Append(transToEN, transToEN)

	win.ComboTransTo.Connect("changed", func() {
		//log.Printf("Lang to: %s", win.ComboTransTo.GetActiveID())
		win.setLocale(win.ComboTransTo.GetActiveID())
	})

	win.LblTransTo, err = gtk.LabelNew("Translate to")
	checkErr(err)

	win.darkToggle, err = gtk.ToggleButtonNew()
	checkErr(err)

	param, err := gtk.SettingsGetDefault()
	checkErr(err)

	win.darkToggle.Connect("toggled", func() {
		if win.darkToggle.GetActive() {
			param.SetProperty("gtk-application-prefer-dark-theme", 1)
		} else {
			param.SetProperty("gtk-application-prefer-dark-theme", 0)
		}

	})

	// построение UI
	//Основные элеиенты
	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	checkErr(err)
	// Нижняя полоса
	boxFooter, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 3)
	checkErr(err)
	boxFooter.SetMarginTop(3)    // отступ сверху
	boxFooter.SetMarginBottom(3) // отступ снизу
	boxFooter.SetMarginStart(3)  // отступ слева
	boxFooter.SetMarginEnd(3)    // отступ справа

	// Кнопки
	boxBtn, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 3)
	checkErr(err)

	boxCfg, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	checkErr(err)

	sep1, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	checkErr(err)
	sep1.SetHExpand(true)

	sep2, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	checkErr(err)

	sep3, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	checkErr(err)

	box.Add(scroll)
	box.Add(boxFooter)

	boxFooter.Add(boxCfg)
	boxFooter.Add(sep1)
	boxFooter.Add(boxBtn)

	boxBtn.Add(win.BtnNew)
	boxBtn.Add(win.BtnClear)
	boxBtn.Add(win.BtnExit)

	boxBtn.SetHAlign(gtk.ALIGN_END) // расположение элементов по горизонтали
	boxBtn.SetSpacing(10)           // интервал между элементами
	boxBtn.SetHomogeneous(true)

	boxCfg.Add(win.SpinBtn)
	boxCfg.Add(sep2)
	boxCfg.Add(win.LblTransTo)
	boxCfg.Add(win.ComboTransTo)
	boxCfg.Add(sep3)
	boxCfg.Add(win.darkToggle)

	win.SpinBtn.SetHAlign(gtk.ALIGN_START)
	//win.BtnNew.SetHAlign(gtk.ALIGN_START)
	//win.BtnExit.SetHAlign(gtk.ALIGN_END)
	win.BtnNew.SetVisible(false)

	win.Window.Add(box)

	// Set the default window size.
	//win.Window.SetDefaultSize(800, 600)
	//win.Window.SetPosition(gtk.WIN_POS_CENTER)

	return win
}

// Определяем лог файл и запускаем чтение в горутине
func loadAndRun(mainUI *MainWindow) {

	potbs_logfile := "."

	// При недоступности директории или файла логов
	// выводим окно выбора папки с логом
	ok := false
	for !ok {
		_, err := os.Stat(mainUI.pathToLog)
		if err == nil {
			// получаем имя файла лога
			potbs_logfile = getLastLog(filepath.Clean(mainUI.pathToLog))
			if potbs_logfile != "." {
				ok = true
				break
			}
		}

		dialog, err := gtk.FileChooserNativeDialogNew("Выберите директорию с логами PotBS", mainUI.Window, gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER, "OK", "Отмена")
		//dialog, err := gtk.FileChooserDialogNewWith1Button("Выберите директорию с логами PotBS", mainUI.Window, gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER, "OK", gtk.RESPONSE_ACCEPT)
		checkErr(err)
		respons := dialog.Run()

		// NativeDialog возвращает int с кодом ответа. -3 это GTK_RESPONSE_ACCEPT
		if respons != int(gtk.RESPONSE_ACCEPT) {
			//win.Window.Close()
			os.Exit(1)
		}

		mainUI.pathToLog = dialog.GetFilename()
		dialog.Destroy()

	}

	log.Printf("Open file: %s", potbs_logfile)
	// Прописываем имя файла в заголовок
	mainUI.Window.SetTitle(title + " (" + filepath.Base(potbs_logfile) + ")")

	// Запускаем чтение лог файла
	mainUI.tailQuit = false
	go mainUI.tailLog(filepath.Clean(potbs_logfile))

}

func (mainUI *MainWindow) tailLog(dir string) {

	// регулярка поиска времени
	re := regexp.MustCompile(`\d\d:\d\d:\d\d`)
	var iter *gtk.TreeIter

	t, err := tail.TailFile(dir, tail.Config{Follow: true, MustExist: true, Poll: true}) //Poll: true,
	checkErr(err)
	// Текущее время  -10 сек.
	// чтобы первый запрос прошел без задержки
	lastT := time.Now().Add(-(10 * time.Second))
	//
	for line := range t.Lines {
		// Задержка отправки сообщения раз в 3 сек
		sec := time.Now().Sub(lastT).Seconds()
		if sec < mainUI.SpinBtn.GetValue() {
			//log.Printf("Sleep %.2f sec.", mainUI.delaySending-sec)
			time.Sleep(time.Duration(mainUI.SpinBtn.GetValue()-sec) * time.Second)
		}

		if mainUI.tailQuit {
			log.Println("Quit goroutine")
			return
		}

		// Строки не требующие вывода
		switch {
		case str.Contains(line.Text, "Aliased memory pool:"):
			continue
		case str.Contains(line.Text, "Total allocated:"):
			continue
		case str.Contains(line.Text, "Total freed:"):
			continue
		case str.Contains(line.Text, "Net allocated:"):
			continue
		case str.Contains(line.Text, "Net allocated high:"):
			continue
		case str.Contains(line.Text, "Total allocated with overhead:"):
			continue
		case str.Contains(line.Text, "Total freed with overhead:"):
			continue
		case str.Contains(line.Text, "Net allocated with overhead:"):
			continue
		case str.Contains(line.Text, "Net allocated high with overhead:"):
			continue
		case str.Contains(line.Text, "Total allocation count:"):
			continue
		case str.Contains(line.Text, "Total free count:"):
			continue
		case str.Contains(line.Text, "Net allocation count:"):
			continue
		case str.Contains(line.Text, "Net allocation count high:"):
			continue
		case str.Contains(line.Text, "Total pool arena size:"):
			continue
		case str.Contains(line.Text, "Pre size:"):
			continue
		case str.Contains(line.Text, "Max size limit:"):
			continue
		case str.Contains(line.Text, "Minimum increment:"):
			continue
		case str.Contains(line.Text, "Raw address:"):
			continue
		case str.Contains(line.Text, "Lowest address:"):
			continue
		case str.Contains(line.Text, "Highest address:"):
			continue
		case str.Contains(line.Text, "Span:"):
			continue
		case str.Contains(line.Text, "Pool index:"):
			continue
		case str.Contains(line.Text, "Pool flags:"):
			continue
		case str.Contains(line.Text, "Check level:"):
			continue
		case str.Contains(line.Text, "Message Level:"):
			continue
		case str.Contains(line.Text, "igAliasMemoryPool:"):
			continue

		}

		// Пока проверяем оба.
		// Т.к. выбранный язык перевода не гарантирует такой же язык интерфейса игры
		// Строки не требующие перевода (стандартные сообщения)
		if isNotReqTranslationRU(line.Text) {
			continue
		}
		if isNotReqTranslationEN(line.Text) {
			continue
		}

		// Есть ли в строке время? Получаем позицию времени
		timeInd := re.FindStringIndex(line.Text)
		// Если позиции нет, значит продолжается старая строка
		if timeInd == nil {
			// Получаем текущий текст
			currentVal, err := mainUI.ListStore.GetValue(iter, COLUMN_TEXT)
			checkErr(err)
			currentText, err := currentVal.GetString()
			checkErr(err)

			// переводим новый
			trtext, err := translate(line.Text, "auto", str.ToLower(mainUI.ComboTransTo.GetActiveID()))
			if err == nil {
				mainUI.ListStore.SetValue(iter, COLUMN_TEXT, currentText+"\n"+trtext)
			} else {
				log.Println(err.Error())
				mainUI.ListStore.SetValue(iter, COLUMN_TEXT, currentText+"\n"+line.Text)
			}
			lastT = time.Now()
			continue
		}

		// Только для сообщений чата
		chatpos := str.Index(line.Text, "Chat_Messages: ")

		if chatpos != -1 {
			// только если найден канал (отсеиваем служебные)
			NeedTranslate := false
			chanels, err := getChanelList("all")
			if err == nil {
				for _, chanel := range chanels {
					if str.Contains(line.Text, chanel) {
						NeedTranslate = true
					}

				}
			}

			if NeedTranslate {
				iter = mainUI.ListStore.Append()
				// add text
				text := line.Text[chatpos+len("Chat_Messages: "):]
				text = str.TrimSpace(text)
				// переводим
				trtext, err := translate(text, "auto", str.ToLower(mainUI.ComboTransTo.GetActiveID()))
				if err == nil {
					mainUI.ListStore.SetValue(iter, COLUMN_TEXT, trtext)
				} else {
					log.Println(err.Error())
					mainUI.ListStore.SetValue(iter, COLUMN_TEXT, text)
				}

				//add time
				mainUI.ListStore.SetValue(iter, COLUMN_DATE, line.Text[timeInd[0]:timeInd[0]+8])
				lastT = time.Now()
				continue
			}
		}
	}
}

// Append a row to the list store for the tree view
func addRow(listStore *gtk.ListStore, id, tpe, en, ru string) error {
	// Get an iterator for a new row at the end of the list store
	iter := listStore.Append()

	// Set the contents of the list store row that the iterator represents
	err := listStore.Set(iter,
		[]int{0, 1, 2, 3},
		[]interface{}{id, tpe, en, ru})
	if err != nil {
		log.Fatal("[ERR]\tUnable to add row:", err)
	}

	return err

}

// Add a column to the tree view (during the initialization of the tree view)
// We need to distinct the type of data shown in either column.
func createTextColumn(title string, id int) *gtk.TreeViewColumn {
	// In this column we want to show text, hence create a text renderer
	cellRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		log.Fatal("Unable to create text cell renderer:", err)
	}

	// Tell the renderer where to pick input from. Text renderer understands
	// the "text" property.
	column, err := gtk.TreeViewColumnNewWithAttribute(title, cellRenderer, "text", id)
	if err != nil {
		log.Fatal("Unable to create cell column:", err)
	}

	return column
}

// Поиск n-го вхождения substr в str
// Возвращает номер позиции или -1, если такого вхождения нет.
func indexN(strg, substr string, n int) int {

	ind := 0
	pos := 0
	for i := 0; i < n; i++ {
		ind = str.Index(strg[pos:], substr)
		if ind == -1 {
			return ind
		}
		pos += ind
		pos += len(substr)
	}

	return pos
}

// Получаем файл лога с последней модификацией
func getLastLog(dir string) string {

	files, _ := filepath.Glob(filepath.Join(dir, "PotBS_*.txt"))
	var newestFile string
	var newestTime int64 = 0
	for _, f := range files {
		fi, err := os.Stat(f)
		checkErr(err)
		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFile = f
		}
	}
	return filepath.Clean(newestFile)
}

func translate(source, sourceLang, targetLang string) (string, error) {
	var trtext string

	if len(source) < 3 {
		return "", errors.New("empty source")
	}

	// Убираем названия каналов, чтобы они не фигурировали в переводе
	var replChanel, replName replacement

	chanels, err := getChanelList("all")
	if err == nil {
		for _, chanel := range chanels {
			ind := str.Index(source, chanel)
			if ind != -1 {
				replChanel.text = chanel
				replChanel.start = ind
				replChanel.lenght = len(chanel)
			}
		}
	}

	if replChanel.lenght > 0 {
		source = source[:replChanel.start] + source[replChanel.start+replChanel.lenght:]
	}

	// Убираем имя, чтобы его не переводить
	re_macros := regexp.MustCompile(`\[(.+?)\]`)
	macros := re_macros.FindStringIndex(source)
	if macros != nil {
		replName.text = source[macros[0]:macros[1]]
		replName.start = macros[0]
		replName.lenght = len(replName.text)
		source = source[:macros[0]] + source[macros[1]:]
	}

	// Оптимизация кол-ва запросов.
	// Если переводи на русский, а строка уже на русском, то возвращаем ее
	if str.ToLower(targetLang) == "ru" {
		match, _ := regexp.MatchString(`\p{Latin}`, source)
		if !match {
			//return "", errors.New("Не содержит латиницу")
			trtext = source
			if replName.lenght > 0 {
				trtext = trtext[:replName.start] + replName.text + trtext[replName.start:]
			}
			if replChanel.lenght > 0 {
				trtext = trtext[:replChanel.start] + replChanel.text + trtext[replChanel.start:]
			}
			return trtext, nil
		}
	}
	// Сначала переводим в гугле
	trtext, err = gotr.Translate(source, sourceLang, targetLang)
	if err == nil {
		if replName.lenght > 0 {
			trtext = trtext[:replName.start] + replName.text + trtext[replName.start:]
		}
		if replChanel.lenght > 0 {
			trtext = trtext[:replChanel.start] + replChanel.text + trtext[replChanel.start:]
		}
		return trtext, nil
	}

	// Раз не вышли, значит пробуем через libretranslate

	trtext, err = libre.Translate(source, sourceLang, targetLang)
	if err == nil {
		if replName.lenght > 0 {
			trtext = trtext[:replName.start] + replName.text + trtext[replName.start:]
		}
		if replChanel.lenght > 0 {
			trtext = trtext[:replChanel.start] + replChanel.text + trtext[replChanel.start:]
		}
		return trtext, nil
	}

	return "", err
}

// Строка не требует перевода
func isNotReqTranslationRU(sourse string) bool {
	// Строки не требующие перевода
	switch {
	case str.Contains(sourse, "Вы переключились на канал"):
		return true
	case str.Contains(sourse, "Вы присоединились к каналу"):
		return true
	case str.Contains(sourse, "Вы изучили навык"):
		return true
	case str.Contains(sourse, "Вы получили новый"):
		return true
	case str.Contains(sourse, "был помещен в ваш сухой док"):
		return true
	}
	return false
}

// Строка не требует перевода
func isNotReqTranslationEN(sourse string) bool {
	// Строки не требующие перевода
	switch {
	case str.Contains(sourse, "You have changed to the"):
		return true
	case str.Contains(sourse, "You have joined the"):
		return true
	case str.Contains(sourse, "You learned"):
		return true
	case str.Contains(sourse, "You gained a level"):
		return true
	case str.Contains(sourse, "placed in your dockyard"):
		return true
	}
	return false
}

func getChanelList(lang string) ([]string, error) {

	listRU := []string{"[Нация] ", "[сообщество] ", "[Местный] ", "[Группа] ", "[Локальный] ", "[Торговля] ",
		"[Битва] ", "[Схватка] ", "[Мероприятия] ", "[Live Events] ", "[Зона] ",
		" говорит вам:", "Вы говорите игроку ",
		"[Сообщение дня в порту]:", "[Системное сообщение дня]:", "[Сообщение дня сообщества]:",
	}
	listEN := []string{"[Nation] ", "[Society] ", "[Local] ", "[Group] ", "[Trade] ",
		"[Battle] ", "[Skirmish] ", "[Live Events] ", "[Area] ",
		" tells you:", "You tell ",
		"[Port message of the day]:", "[System message of the day]:", "[Society message of the day]:",
	}
	switch str.ToLower(lang) {
	case "ru":
		return listRU, nil
	case "en":
		return listEN, nil
	case "all":
		return append(listRU, listEN...), nil
	}

	return nil, errors.New("unknown lang")
}

func (mainUI *MainWindow) setLocale(locale string) {
	switch str.ToLower(locale) {
	case "ru":
		mainUI.LblTransTo.SetLabel("Переводить на ")
		mainUI.BtnNew.SetLabel("Переоткрыть")
		mainUI.BtnClear.SetLabel("Очистить")
		mainUI.BtnExit.SetLabel("Закрыть")
		mainUI.SpinBtn.SetTooltipText("Мин. интервал между сообщениями (в сек.)\nЧтобы не превысить ограничения сервера gtranslate")
		mainUI.darkToggle.SetLabel("Темный")
	case "en":
		mainUI.LblTransTo.SetLabel("Translate to ")
		mainUI.BtnNew.SetLabel("Reload")
		mainUI.BtnClear.SetLabel("Clear")
		mainUI.BtnExit.SetLabel("Quit")
		mainUI.SpinBtn.SetTooltipText("Min. interval between messages (in seconds)\nIn order not to exceed the gtranslate server limits")
		mainUI.darkToggle.SetLabel("Dark")
	}
}
func checkErr(e error, text_opt ...string) {
	if e != nil {

		if len(text_opt) > 0 {
			log.Println(text_opt[0])
		}
		// panic for any errors.
		//log.Panic(e)
		log.Fatal(e, e.Error())
	}
}
