package main

import (
	_ "embed"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	WS_OVERLAPPEDWINDOW          = 0x00CF0000
	WS_VISIBLE                   = 0x10000000
	WS_CHILD                     = 0x40000000
	WS_BORDER                    = 0x00800000
	ES_MULTILINE                 = 0x0004
	ES_AUTOVSCROLL               = 0x0040
	WS_EX_CLIENTEDGE             = 0x00000200
	WM_CLOSE                     = 0x0010
	WS_VSCROLL                   = 0x00200000
	WS_TABSTOP                   = 0x00010000
	ES_RIGHT                     = 0x0002
	ES_LEFT                      = 0
	ES_AUTOHSCROLL               = 0x0080
	LBS_NOTIFY                   = 0x0001
	WM_CREATE                    = 1
	WM_DESTROY                   = 2
	WM_COMMAND                   = 0x111
	WM_NOTIFY                    = 0x4E
	WM_SETFONT                   = 0x30
	SS_RIGHT                     = 0x0002
	SS_RIGHTJUST                 = 0x0400
	WS_EX_RTLREADING             = 0x2000
	WS_EX_LAYOUTRTL              = 0x400000
	WS_EX_NOINHERITLAYOUT        = 0x00100000
	BS_RIGHT                     = 0x00000200
	BS_RIGHTBUTTON               = 0x00000020
	WM_SETICON                   = 0x80
	WM_GETTEXT                   = 0xD
	WM_GETTEXTLENGTH             = 0xE
	WM_SETTEXT                   = 0xC
	LB_ADDSTRING                 = 0x180
	LB_RESETCONTENT              = 0x184
	LB_GETCURSEL                 = 0x188
	LB_SETCURSEL                 = 0x186
	LVM_FIRST                    = 0x1000
	LVM_GETHEADER                = LVM_FIRST + 31
	LVM_INSERTCOLUMNW            = LVM_FIRST + 97
	LVM_INSERTITEMW              = LVM_FIRST + 77
	LVM_SETITEMTEXTW             = LVM_FIRST + 116
	LVM_DELETEALLITEMS           = LVM_FIRST + 9
	LVM_GETNEXTITEM              = LVM_FIRST + 12
	LVM_SETEXTENDEDLISTVIEWSTYLE = LVM_FIRST + 54
	LVS_REPORT                   = 1
	LVS_SINGLESEL                = 4
	LVS_SHOWSELALWAYS            = 8
	LVS_EX_GRIDLINES             = 1
	LVS_EX_FULLROWSELECT         = 0x20
	LVNI_SELECTED                = 2
	LVCF_FMT                     = 1
	LVCF_WIDTH                   = 2
	LVCF_TEXT                    = 4
	LVCFMT_RIGHT                 = 1
	LVCFMT_LEFT                  = 0
	CBS_DROPDOWNLIST             = 3
	CB_ADDSTRING                 = 0x143
	CB_GETCURSEL                 = 0x147
	CB_SETCURSEL                 = 0x14E
	HDM_FIRST                    = 0x1200
	HDM_GETITEMW                 = HDM_FIRST + 11
	HDM_SETITEMW                 = HDM_FIRST + 12
	HDI_FORMAT                   = 4
	HDF_JUSTIFYMASK              = 3
	HDF_RIGHT                    = 1
	MB_OK                        = 0
	MB_ICONERROR                 = 0x10
	MB_ICONINFORMATION           = 0x40
	MB_YESNO                     = 4
	IDYES                        = 6
	SW_SHOW                      = 5
	OFN_EXPLORER                 = 0x80000
	OFN_PATHMUSTEXIST            = 0x800
	OFN_FILEMUSTEXIST            = 0x1000
	OFN_OVERWRITEPROMPT          = 2
	ES_READONLY                  = 0x0800
	WS_POPUP                     = 0x80000000
	WS_CAPTION                   = 0x00C00000
	WS_SYSMENU                   = 0x00080000
	WS_THICKFRAME                = 0x00040000
	WS_MINIMIZEBOX               = 0x00020000
	WS_MAXIMIZEBOX               = 0x00010000
	WM_NCHITTEST                 = 0x0084
	WM_SIZE                      = 0x0005
	WM_PAINT                     = 0x000F
	WM_ERASEBKGND                = 0x0014
	WM_CTLCOLORSTATIC            = 0x0138
	TRANSPARENT                  = 1
	HTCLIENT                     = 1
	HTCAPTION                    = 2
	SW_MINIMIZE                  = 6
	SW_MAXIMIZE                  = 3
	SW_RESTORE                   = 9
	SW_HIDE                      = 0
)

var (
	user32                            = syscall.NewLazyDLL("user32.dll")
	kernel32                          = syscall.NewLazyDLL("kernel32.dll")
	gdi32                             = syscall.NewLazyDLL("gdi32.dll")
	comdlg32                          = syscall.NewLazyDLL("comdlg32.dll")
	createWindow                      = user32.NewProc("CreateWindowExW")
	defWindow                         = user32.NewProc("DefWindowProcW")
	sendMessage                       = user32.NewProc("SendMessageW")
	messageBox                        = user32.NewProc("MessageBoxW")
	hwnd                              uintptr
	mainIcon                          uintptr
	captionBrush                      uintptr
	captionChildren                   map[uintptr]bool
	hlist                             uintptr
	fields                            [4]uintptr
	entries                           [5]uintptr
	summary                           uintptr
	font                              uintptr
	titleFont                         uintptr
	picker                            uintptr
	pickerTitle                       uintptr
	pickerYear, pickerMonth           int
	pickerDays                        [31]uintptr
	pickerYearCombo, pickerMonthCombo uintptr
	embeddedFontHandle                uintptr
	embeddedFontCount                 uint32
	codeOptions                       []string
	headers                           = [6]string{"", "14050830", "000000001", "0", "0", ""}
	rows                              []Record
	currentFile                       string
	helpDialog, aboutDialog           uintptr
	infoClassRegistered               bool
)

//go:embed Vazirmatn-Regular.ttf
var embeddedVazirmatn []byte

func loadEmbeddedFont() bool {
	if len(embeddedVazirmatn) == 0 {
		return false
	}
	// Windows AddFontMemResourceEx registers this font privately for this process.
	// Keep the backing slice alive until application shutdown.
	handle := call(gdi32.NewProc("AddFontMemResourceEx"), uintptr(unsafe.Pointer(&embeddedVazirmatn[0])), uintptr(uint32(len(embeddedVazirmatn))), 0, uintptr(unsafe.Pointer(&embeddedFontCount)))
	if handle == 0 {
		return false
	}
	embeddedFontHandle = handle
	return true
}

type Record struct {
	IBAN, Name string
	Amount     int64
	Code, Desc string
	Extra      [4]string
}

func ptr(s string) *uint16                              { return syscall.StringToUTF16Ptr(s) }
func uintStr(u []uint16) string                         { return syscall.UTF16ToString(u) }
func call(p *syscall.LazyProc, args ...uintptr) uintptr { x, _, _ := p.Call(args...); return x }
func low(v uintptr) int                                 { return int(uint16(v)) }
func window(class, title string, style uint32, x, y, w, h int, id int, parent uintptr) uintptr {
	ex := uintptr(0)
	if parent == hwnd && hwnd != 0 {
		ex = 0 // Do not inherit or request mirroring of coordinates from the top-level window.
		// RTL-reading is applied before window creation to prevent late mirroring/repaint artifacts.
		if class == "STATIC" || class == "BUTTON" || class == "COMBOBOX" || (class == "EDIT" && style&ES_RIGHT != 0) {
			ex |= WS_EX_RTLREADING
		}
	}
	return call(createWindow, ex, uintptr(unsafe.Pointer(ptr(class))), uintptr(unsafe.Pointer(ptr(title))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, uintptr(id), call(kernel32.NewProc("GetModuleHandleW"), 0), 0)
}
func ctl(class, title string, x, y, w, h, id int, style uint32) uintptr {
	if class == "STATIC" {
		style |= SS_RIGHT
	}
	if class == "BUTTON" {
		style |= BS_RIGHT
	}
	if class == "EDIT" && (style&ES_LEFT) != 0 { /* numeric and account fields intentionally LTR */
	}
	v := window(class, title, WS_CHILD|WS_VISIBLE|style, x, y, w, h, id, hwnd)
	call(sendMessage, v, WM_SETFONT, font, 1)
	return v
}
func set(h uintptr, s string) { call(sendMessage, h, WM_SETTEXT, 0, uintptr(unsafe.Pointer(ptr(s)))) }
func get(h uintptr) string {
	n := int(call(sendMessage, h, WM_GETTEXTLENGTH, 0, 0))
	b := make([]uint16, n+2)
	call(sendMessage, h, WM_GETTEXT, uintptr(len(b)), uintptr(unsafe.Pointer(&b[0])))
	return uintStr(b)
}
func alert(s string, err bool) {
	flag := uintptr(MB_ICONINFORMATION)
	if err {
		flag = MB_ICONERROR
	}
	call(messageBox, hwnd, uintptr(unsafe.Pointer(ptr(s))), uintptr(unsafe.Pointer(ptr("برنامه غیر رسمی واریزهای پایا گروهی بانک ملت"))), flag)
}
func ask(s string) bool {
	return call(messageBox, hwnd, uintptr(unsafe.Pointer(ptr(s))), uintptr(unsafe.Pointer(ptr("تأیید"))), MB_YESNO) == IDYES
}
func normalize(s string) string {
	r := strings.NewReplacer("۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9", "٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9", ",", "", "٬", "")
	return strings.TrimSpace(r.Replace(s))
}
func num(s string) (int64, error) { return strconv.ParseInt(normalize(s), 10, 64) }
func validIBAN(s string) bool {
	s = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	if !regexp.MustCompile(`^IR[0-9]{24}$`).MatchString(s) {
		return false
	}
	s = s[4:] + s[:4]
	var rem int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			rem = (rem*10 + int(c-'0')) % 97
		} else {
			v := int(c-'A') + 10
			rem = (rem*100 + v) % 97
		}
	}
	return rem == 1
}
func sum() int64 {
	var n int64
	for _, r := range rows {
		n += r.Amount
	}
	return n
}
func comma(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// Native common-controls ListView: report mode, RTL layout, right-aligned text, and grid lines.
type lvColumn struct {
	Mask       uint32
	Fmt        int32
	Cx         int32
	Text       *uint16
	TextMax    int32
	SubItem    int32
	Image      int32
	Order      int32
	MinWidth   int32
	DefWidth   int32
	IdealWidth int32
}
type lvItem struct {
	Mask      uint32
	Item      int32
	SubItem   int32
	State     uint32
	StateMask uint32
	Text      *uint16
	TextMax   int32
	Image     int32
	Param     uintptr
	Indent    int32
	GroupID   int32
	Columns   uint32
	PColumns  *uint32
	PColFmt   *int32
	Group     int32
}
type hdItem struct {
	Mask    uint32
	Cxy     int32
	Text    *uint16
	Bitmap  uintptr
	TextMax int32
	Fmt     int32
	Param   uintptr
	Image   int32
	Order   int32
	Type    uint32
	Filter  uintptr
	State   uint32
}
type nmHeader struct {
	HwndFrom uintptr
	IDFrom   uintptr
	Code     int32
	Padding  uint32
}

func columnAlignment(i int) int32 {
	if i == 0 || i == 2 || i == 3 {
		return LVCFMT_LEFT
	}
	return LVCFMT_RIGHT
}
func initTable() {
	titles := []string{"ردیف", "نام دریافت‌کننده", "مبلغ", "شماره حساب", "بابت", "توضیحات"}
	widths := []int32{58, 165, 135, 235, 230, 267}
	for i, t := range titles {
		u := ptr(t)
		col := lvColumn{Mask: LVCF_FMT | LVCF_WIDTH | LVCF_TEXT, Fmt: columnAlignment(i), Cx: widths[i], Text: u}
		call(sendMessage, hlist, LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&col)))
		runtime.KeepAlive(u)
	}
	call(sendMessage, hlist, LVM_SETEXTENDEDLISTVIEWSTYLE, uintptr(LVS_EX_GRIDLINES|LVS_EX_FULLROWSELECT), uintptr(LVS_EX_GRIDLINES|LVS_EX_FULLROWSELECT))
	// Header alignment is independent from the ListView cell alignment.
	// All captions: right aligned; IBAN / amount / row numbers: left aligned cells.
	header := call(sendMessage, hlist, LVM_GETHEADER, 0, 0)
	if header != 0 {
		// The ListView data is RTL, but the native header is a separate Win32
		// window. Explicitly mirror its column layout to match the data cells.
		// Do not mirror the parent form or change column/subitem indices.
		const gwlExStyle = ^uintptr(19) // GWL_EXSTYLE = -20
		getLong := user32.NewProc("GetWindowLongPtrW")
		setLong := user32.NewProc("SetWindowLongPtrW")
		oldEx := call(getLong, header, gwlExStyle)
		if oldEx&WS_EX_LAYOUTRTL == 0 {
			call(setLong, header, gwlExStyle, oldEx|WS_EX_LAYOUTRTL|WS_EX_RTLREADING)
			const SWP_NOSIZE = 0x0001
			const SWP_NOMOVE = 0x0002
			const SWP_NOZORDER = 0x0004
			const SWP_FRAMECHANGED = 0x0020
			call(user32.NewProc("SetWindowPos"), header, 0, 0, 0, 0, 0, SWP_NOSIZE|SWP_NOMOVE|SWP_NOZORDER|SWP_FRAMECHANGED)
		}
		for i := range titles {
			item := hdItem{Mask: HDI_FORMAT}
			if call(sendMessage, header, HDM_GETITEMW, uintptr(i), uintptr(unsafe.Pointer(&item))) != 0 {
				item.Fmt = (item.Fmt &^ HDF_JUSTIFYMASK) | HDF_RIGHT
				call(sendMessage, header, HDM_SETITEMW, uintptr(i), uintptr(unsafe.Pointer(&item)))
			}
		}
	}
}
func rowCell(i, j int, value string) {
	u := ptr(value)
	item := lvItem{Mask: 1, Item: int32(i), SubItem: int32(j), Text: u}
	if j == 0 {
		call(sendMessage, hlist, LVM_INSERTITEMW, 0, uintptr(unsafe.Pointer(&item)))
	} else {
		call(sendMessage, hlist, LVM_SETITEMTEXTW, uintptr(i), uintptr(unsafe.Pointer(&item)))
	}
	runtime.KeepAlive(u)
}
func refresh() {
	// The displayed total and the exported TXT/CCTI header always derive from current rows.
	total := sum()
	headers[3] = strconv.Itoa(len(rows))
	headers[4] = strconv.FormatInt(total, 10)
	call(sendMessage, hlist, LVM_DELETEALLITEMS, 0, 0)
	for i, r := range rows {
		cells := []string{strconv.Itoa(i + 1), r.Name, comma(r.Amount), r.IBAN, shortCodeLabel(r.Code), r.Desc}
		for j, v := range cells {
			rowCell(i, j, v)
		}
	}
	set(summary, fmt.Sprintf("تعداد واریزها: %d       جمع کل: %s ریال", len(rows), comma(total)))
}
func sel() int {
	v := call(sendMessage, hlist, LVM_GETNEXTITEM, ^uintptr(0), LVNI_SELECTED)
	if v == ^uintptr(0) {
		return -1
	}
	return int(v)
}

// Labels are transcribed from the screenshot of the original banking program.
// Codes 1..11 are cross-checked against the provided TXT file.
var originalBankLabels = map[string]string{
	"1": "واریز حقوق", "2": "امور بیمه خدمات", "3": "امور درمانی",
	"4": "امور سرمایه‌گذاری و بورس", "5": "امور ارزی در چارچوب ضوابط و مقررات",
	"6": "پرداخت قرض و تأدیه دیون", "7": "امور بازنشستگی",
	"8": "معاملات اموال منقول", "9": "معاملات اموال غیرمنقول",
	"10": "مدیریت نقدینگی", "11": "عوارض گمرکی", "12": "تسویه مالیاتی",
	"13": "سایر خدمات دولتی", "14": "تسهیلات و تعهدات", "15": "تودیع وثیقه",
	"16": "هزینه عمومی و امور روزمره", "17": "کمک‌های خیریه", "18": "خرید کالا", "19": "خرید خدمات",
}

func codeLabel(code string) string {
	if label, ok := originalBankLabels[normalize(code)]; ok {
		return label
	}
	return "علت انتقال نامشخص"
}
func shortCodeLabel(code string) string {
	if label, ok := originalBankLabels[normalize(code)]; ok {
		return label
	}
	return "علت انتقال نامشخص"
}
func ensureCode(code string) int {
	code = normalize(code)
	for i, v := range codeOptions {
		if v == code {
			return i
		}
	}
	codeOptions = append(codeOptions, code)
	u := ptr(codeLabel(code))
	call(sendMessage, entries[3], CB_ADDSTRING, 0, uintptr(unsafe.Pointer(u)))
	runtime.KeepAlive(u)
	return len(codeOptions) - 1
}
func selectCode(code string) {
	idx := ensureCode(code)
	call(sendMessage, entries[3], CB_SETCURSEL, uintptr(idx), 0)
}
func currentCode() string {
	idx := int(call(sendMessage, entries[3], CB_GETCURSEL, 0, 0))
	if idx < 0 || idx >= len(codeOptions) {
		return ""
	}
	return codeOptions[idx]
}
func onSelect() {
	i := sel()
	if i < 0 || i >= len(rows) {
		return
	}
	r := rows[i]
	v := []string{r.IBAN, r.Name, strconv.FormatInt(r.Amount, 10), r.Code, r.Desc}
	for j := range entries {
		if j == 3 {
			selectCode(v[j])
		} else {
			set(entries[j], v[j])
		}
	}
}
func addOrUpdate(update bool) {
	v := make([]string, 5)
	for j := range entries {
		if j == 3 {
			v[j] = currentCode()
		} else {
			v[j] = strings.TrimSpace(get(entries[j]))
		}
	}
	if v[3] == "" {
		alert("علت انتقال را انتخاب کنید.", true)
		return
	}
	a, e := num(v[2])
	if e != nil || a <= 0 || a > 5_000_000_000 {
		alert("مبلغ باید بین ۱ و ۵٬۰۰۰٬۰۰۰٬۰۰۰ ریال (۵۰۰ میلیون تومان) باشد.", true)
		return
	}
	if !validIBAN(v[0]) {
		alert("شماره شبای مقصد معتبر نیست.", true)
		return
	}
	if v[1] == "" {
		alert("نام دریافت‌کننده وارد نشده است.", true)
		return
	}
	r := Record{IBAN: strings.ToUpper(strings.ReplaceAll(v[0], " ", "")), Name: v[1], Amount: a, Code: v[3], Desc: v[4]}
	if update {
		i := sel()
		if i < 0 {
			alert("ابتدا یک ردیف را انتخاب کنید.", true)
			return
		}
		r.Extra = rows[i].Extra
		rows[i] = r
	} else {
		rows = append(rows, r)
	}
	refresh()
}
func deleteSelected() {
	i := sel()
	if i < 0 {
		return
	}
	if ask("ردیف انتخاب‌شده حذف شود؟") {
		rows = append(rows[:i], rows[i+1:]...)
		refresh()
	}
}

// This is offline IBAN structural / MOD-97 validation, not a bank account inquiry.
func checkAccounts() {
	readHeaders()
	bad := make([]string, 0)
	if !validIBAN(headers[0]) {
		bad = append(bad, "شبای حساب مبدأ معتبر نیست: "+headers[0])
	}
	for i, r := range rows {
		if !validIBAN(r.IBAN) {
			bad = append(bad, fmt.Sprintf("ردیف %d: شبای مقصد نامعتبر است (%s)", i+1, r.IBAN))
		}
	}
	// The destination IBAN currently in the input field is checked as well.
	pending := strings.TrimSpace(get(entries[0]))
	if pending != "" && !validIBAN(pending) {
		bad = append(bad, "شبای واردشده در فرم مقصد معتبر نیست: "+pending)
	}
	if len(bad) == 0 {
		alert(fmt.Sprintf("کنترل ساختاری شبا موفق بود.\n\nتعداد ردیف‌های بررسی‌شده: %d\nشبای مبدأ: معتبر\nشماره شبای مقصد: همه ردیف‌ها معتبر\n\nتوجه: این کنترل فقط طول، قالب IR و رقم کنترل MOD-97 را بررسی می‌کند؛ وجود حساب، فعال‌بودن آن و مطابقت نام صاحب حساب با بانک را تأیید نمی‌کند.", len(rows)), false)
		return
	}
	limit := len(bad)
	if limit > 20 {
		limit = 20
	}
	msg := fmt.Sprintf("%d مورد خطا در کنترل ساختاری شبا یافت شد:\n\n%s", len(bad), strings.Join(bad[:limit], "\n"))
	if len(bad) > limit {
		msg += fmt.Sprintf("\n... و %d خطای دیگر", len(bad)-limit)
	}
	msg += "\n\nاین بررسی، استعلام حساب از بانک نیست."
	alert(msg, true)
}

// Persian (Solar Hijri) calendar. Conversion follows the arithmetic Jalali calendar
// for the supported 1300-1600 year range, so leap Esfand is also checked.
func jalaliGregorian(jy, jm, jd int) (int, int, int) {
	jy += 1595
	days := -355668 + 365*jy + (jy/33)*8 + ((jy%33)+3)/4 + jd
	if jm < 7 {
		days += (jm - 1) * 31
	} else {
		days += 186 + (jm-7)*30
	}
	gy := 400 * (days / 146097)
	days %= 146097
	if days > 36524 {
		gy += 100 * ((days - 1) / 36524)
		days = (days - 1) % 36524
		if days >= 365 {
			days++
		}
	}
	gy += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		gy += (days - 1) / 365
		days = (days - 1) % 365
	}
	gd := days + 1
	mdays := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if gy%4 == 0 && (gy%100 != 0 || gy%400 == 0) {
		mdays[1] = 29
	}
	gm := 1
	for gm <= 12 && gd > mdays[gm-1] {
		gd -= mdays[gm-1]
		gm++
	}
	return gy, gm, gd
}
func isJalaliLeap(y int) bool {
	gy1, gm1, gd1 := jalaliGregorian(y, 1, 1)
	gy2, gm2, gd2 := jalaliGregorian(y+1, 1, 1)
	d1 := time.Date(gy1, time.Month(gm1), gd1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(gy2, time.Month(gm2), gd2, 0, 0, 0, 0, time.UTC)
	return d2.Sub(d1) == 366*24*time.Hour
}
func monthDays(y, m int) int {
	if m <= 6 {
		return 31
	}
	if m <= 11 {
		return 30
	}
	if isJalaliLeap(y) {
		return 30
	}
	return 29
}
func parseJalali(s string) (int, int, int, error) {
	s = normalize(strings.ReplaceAll(s, "/", ""))
	if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(s) {
		return 0, 0, 0, fmt.Errorf("تاریخ شمسی باید به صورت YYYY/MM/DD باشد")
	}
	y, _ := strconv.Atoi(s[:4])
	m, _ := strconv.Atoi(s[4:6])
	d, _ := strconv.Atoi(s[6:8])
	if y < 1300 || y > 1600 || m < 1 || m > 12 || d < 1 {
		return 0, 0, 0, fmt.Errorf("سال یا ماه یا روز شمسی معتبر نیست")
	}
	if d > monthDays(y, m) {
		return 0, 0, 0, fmt.Errorf("روز انتخاب‌شده برای ماه شمسی معتبر نیست")
	}
	return y, m, d, nil
}
func formattedDate(s string) string {
	s = normalize(strings.ReplaceAll(s, "/", ""))
	if len(s) == 8 {
		return s[:4] + "/" + s[4:6] + "/" + s[6:]
	}
	return s
}

var monthNames = []string{"فروردین", "اردیبهشت", "خرداد", "تیر", "مرداد", "شهریور", "مهر", "آبان", "آذر", "دی", "بهمن", "اسفند"}

func pickerControl(class, title string, x, y, w, h, id int, style uint32) uintptr {
	if class == "STATIC" {
		style |= SS_RIGHT
	}
	if class == "BUTTON" {
		style |= BS_RIGHT
	}
	hnd := window(class, title, WS_CHILD|WS_VISIBLE|style, x, y, w, h, id, picker)
	// Reading direction is assigned during CreateWindowExW, never changed afterward.
	call(sendMessage, hnd, WM_SETFONT, font, 1)
	return hnd
}
func pickerSelectYearMonth() {
	if pickerYearCombo != 0 {
		idx := int(call(sendMessage, pickerYearCombo, CB_GETCURSEL, 0, 0))
		if idx >= 0 && idx <= 25 {
			pickerYear = 1405 + idx
		}
	}
	if pickerMonthCombo != 0 {
		idx := int(call(sendMessage, pickerMonthCombo, CB_GETCURSEL, 0, 0))
		if idx >= 0 && idx < 12 {
			pickerMonth = idx + 1
		}
	}
	drawPicker()
}
func drawPicker() {
	if picker == 0 {
		return
	}
	if pickerYearCombo != 0 {
		call(sendMessage, pickerYearCombo, CB_SETCURSEL, uintptr(pickerYear-1405), 0)
	}
	if pickerMonthCombo != 0 {
		call(sendMessage, pickerMonthCombo, CB_SETCURSEL, uintptr(pickerMonth-1), 0)
	}
	gy, gm, gd := jalaliGregorian(pickerYear, pickerMonth, 1)
	wd := time.Date(gy, time.Month(gm), gd, 0, 0, 0, 0, time.UTC).Weekday()
	offset := (int(wd) + 1) % 7 // Saturday first
	days := monthDays(pickerYear, pickerMonth)
	for i, h := range pickerDays {
		if i < days {
			set(h, fmt.Sprintf("%02d", i+1))
			// Reverse visual columns so Saturday sits on the right in RTL layout.
			col := 6 - (i+offset)%7
			row := (i + offset) / 7
			call(user32.NewProc("SetWindowPos"), h, 0, uintptr(22+col*49), uintptr(114+row*43), 45, 38, 0x0044)
			call(user32.NewProc("ShowWindow"), h, SW_SHOW)
		} else {
			call(user32.NewProc("ShowWindow"), h, 0)
		}
	}
}
func closePicker() {
	if picker != 0 {
		p := picker
		picker = 0
		pickerMonthCombo = 0
		pickerYearCombo = 0
		call(user32.NewProc("DestroyWindow"), p)
	}
}
func changePickerMonth(delta int) {
	pickerMonth += delta
	if pickerMonth < 1 {
		pickerMonth = 12
		pickerYear--
	}
	if pickerMonth > 12 {
		pickerMonth = 1
		pickerYear++
	}
	if pickerYear < 1405 {
		pickerYear = 1405
		pickerMonth = 1
	}
	if pickerYear > 1430 {
		pickerYear = 1430
		pickerMonth = 12
	}
	drawPicker()
}
func openPicker() {
	if picker != 0 {
		call(user32.NewProc("SetForegroundWindow"), picker)
		return
	}
	y, m, _, err := parseJalali(get(fields[1]))
	if err != nil || y < 1405 || y > 1430 {
		y, m = 1405, 8
	}
	pickerYear, pickerMonth = y, m
	picker = window("MellatBatchNativeFa", "انتخاب تاریخ شمسی", WS_POPUP|WS_CAPTION|WS_SYSMENU|WS_VISIBLE, 270, 140, 401, 449, 0, 0)
	// Month and year are real dropdowns; unlike the old picker, either can be selected directly.
	pickerControl("STATIC", "سال", 285, 12, 83, 23, 0, SS_RIGHT)
	pickerYearCombo = pickerControl("COMBOBOX", "", 201, 39, 164, 340, 326, WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST)
	for yr := 1405; yr <= 1430; yr++ {
		u := ptr(fmt.Sprintf("%04d", yr))
		call(sendMessage, pickerYearCombo, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(u)))
		runtime.KeepAlive(u)
	}
	pickerControl("STATIC", "ماه", 110, 12, 83, 23, 0, SS_RIGHT)
	pickerMonthCombo = pickerControl("COMBOBOX", "", 18, 39, 169, 300, 325, WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST)
	for _, month := range monthNames {
		u := ptr(month)
		call(sendMessage, pickerMonthCombo, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(u)))
		runtime.KeepAlive(u)
	}
		for i, name := range []string{"ش", "ی", "د", "س", "چ", "پ", "ج"} {
		pickerControl("STATIC", name, 22+(6-i)*49, 94, 45, 18, 0, SS_RIGHT)
	}
	for i := 0; i < 31; i++ {
		pickerDays[i] = pickerControl("BUTTON", strconv.Itoa(i+1), 0, 0, 45, 38, 400+i, WS_TABSTOP)
	}
	pickerControl("STATIC", "برای انتخاب ماه و سال، از فهرست‌های بالای تقویم استفاده کنید.", 13, 383, 365, 24, 0, SS_RIGHT)
	drawPicker()
}
func choosePickerDay(d int) {
	if picker == 0 || d > monthDays(pickerYear, pickerMonth) {
		return
	}
	set(fields[1], fmt.Sprintf("%04d/%02d/%02d", pickerYear, pickerMonth, d))
	closePicker()
}
func readHeaders() {
	for i, j := range []int{0, 1, 2, 5} {
		headers[j] = strings.TrimSpace(get(fields[i]))
	}
	headers[0] = strings.ToUpper(strings.ReplaceAll(headers[0], " ", ""))
	headers[1] = normalize(strings.ReplaceAll(headers[1], "/", ""))
	headers[2] = normalize(headers[2])
	headers[3] = strconv.Itoa(len(rows))
	headers[4] = strconv.FormatInt(sum(), 10)
}
func validate() error {
	readHeaders()
	if !validIBAN(headers[0]) {
		return fmt.Errorf("شبای مبدأ معتبر نیست")
	}
	if _, _, _, dateErr := parseJalali(headers[1]); dateErr != nil {
		return dateErr
	}
	if !regexp.MustCompile(`^\d{1,9}$`).MatchString(headers[2]) {
		return fmt.Errorf("شماره فایل باید ۱ تا ۹ رقم باشد")
	}
	if headers[5] == "" {
		return fmt.Errorf("نام صاحب حساب مبدأ خالی است")
	}
	if len(rows) == 0 {
		return fmt.Errorf("هیچ ردیفی وجود ندارد")
	}
	for i, r := range rows {
		if !validIBAN(r.IBAN) || r.Amount < 1 || r.Amount > 5_000_000_000 {
			return fmt.Errorf("اشکال در شبا یا مبلغ ردیف %d", i+1)
		}
	}
	return nil
}
func decodeCP1256(b []byte) string {
	out := make([]rune, 0, len(b))
	for _, v := range b {
		if v < 128 {
			out = append(out, rune(v))
		} else {
			out = append(out, cp1256[v-128])
		}
	}
	return string(out)
}
func encodeCP1256(s string) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 128 {
			out = append(out, byte(r))
			continue
		}
		b, ok := cpRev[r]
		if !ok {
			return nil, fmt.Errorf("کاراکتر %q در Windows-1256 قابل ذخیره نیست", r)
		}
		out = append(out, b)
	}
	return out, nil
}
func importTXT(path string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	reader := csv.NewReader(strings.NewReader(decodeCP1256(b)))
	reader.FieldsPerRecord = -1
	all, e := reader.ReadAll()
	if e != nil {
		return e
	}
	if len(all) == 0 || len(all[0]) < 6 {
		return fmt.Errorf("فایل TXT سرآیند معتبر ندارد")
	}
	var rs []Record
	for i, line := range all[1:] {
		if len(line) == 0 {
			continue
		}
		if len(line) < 5 {
			return fmt.Errorf("ردیف %d ناقص است", i+2)
		}
		n, e := num(line[2])
		if e != nil {
			return fmt.Errorf("مبلغ ردیف %d معتبر نیست", i+2)
		}
		r := Record{IBAN: line[0], Name: line[1], Amount: n, Code: line[3], Desc: line[4]}
		for j := 0; j < 4; j++ {
			if len(line) > j+5 {
				r.Extra[j] = line[j+5]
			}
		}
		rs = append(rs, r)
	}
	declaredCount := strings.TrimSpace(all[0][3])
	declaredTotal := strings.TrimSpace(all[0][4])
	copy(headers[:], all[0][:6])
	rows = rs
	for _, r := range rs {
		ensureCode(r.Code)
	}
	for i, j := range []int{0, 1, 2, 5} {
		set(fields[i], headers[j])
		if i == 1 {
			set(fields[i], formattedDate(headers[j]))
		}
	}
	refresh()
	currentFile = path
	actualCount := strconv.Itoa(len(rs))
	actualTotal := strconv.FormatInt(sum(), 10)
	if normalize(declaredCount) != actualCount || normalize(declaredTotal) != actualTotal {
		alert(fmt.Sprintf("هشدار: اطلاعات سرآیند TXT با ردیف‌ها مطابقت ندارد.\n\nتعداد اعلام‌شده: %s | تعداد واقعی: %s\nمجموع اعلام‌شده: %s | مجموع واقعی: %s ریال\n\nبرنامه تعداد و جمع را از ردیف‌ها محاسبه کرده است.", declaredCount, actualCount, declaredTotal, actualTotal), true)
	}
	return nil
}
func exportTXT() (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	if e := w.Write(headers[:]); e != nil {
		return "", e
	}
	for _, r := range rows {
		a := []string{r.IBAN, r.Name, strconv.FormatInt(r.Amount, 10), r.Code, r.Desc}
		a = append(a, r.Extra[:]...)
		if e := w.Write(a); e != nil {
			return "", e
		}
	}
	w.Flush()
	return b.String(), w.Error()
}
func esc(s string) string { var b strings.Builder; xml.EscapeText(&b, []byte(s)); return b.String() }
func exportCCTI() []byte {
	iban, date, seq, total := headers[0], headers[1], headers[2], headers[4]
	name := esc(headers[5])
	datefmt := date[0:4] + "-" + date[4:6] + "-" + date[6:8]
	count := headers[3]
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\" standalone=\"yes\"?>\r\n<Document xmlns=\"urn:iso:std:iso:20022:tech:xsd:CstmrCdtTrfInitn\">\r\n<CstmrCdtTrfInitn>\r\n<GrpHdr>\r\n")
	fmt.Fprintf(&b, "<MsgId>%s</MsgId>\r\n<CreDtTm>%sT%s</CreDtTm>\r\n<NbOfTxs>%s</NbOfTxs>\r\n<CtrlSum>%s</CtrlSum>\r\n<Initgpty>\r\n<Nm>%s</Nm>\r\n</Initgpty>\r\n</GrpHdr>\r\n", esc(iban+seq), datefmt, time.Now().Format("15:04:05"), count, total, name)
	for _, r := range rows {
		fmt.Fprintf(&b, "<PmtInf>\r\n<PmtInfId>1</PmtInfId>\r\n<PmtMtd>TRF</PmtMtd>\r\n<NbOfTxs>%s</NbOfTxs>\r\n<CtrlSum>%s</CtrlSum>\r\n<ReqdExctnDt>%s</ReqdExctnDt>\r\n<Dbtr>\r\n<Nm>%s</Nm>\r\n<Id>\r\n<PrvtId>\r\n<Othr>\r\n<Id>%s</Id>\r\n</Othr>\r\n</PrvtId>\r\n</Id>\r\n</Dbtr>\r\n<DbtrAcct>\r\n<Id>\r\n<IBAN>%s</IBAN>\r\n</Id>\r\n</DbtrAcct>\r\n<DbtrAgt>\r\n<FinInstnId>\r\n<BIC>BMJIIRTH</BIC>\r\n</FinInstnId>\r\n</DbtrAgt>\r\n", count, total, datefmt, name, esc(r.Code), esc(iban))
		fmt.Fprintf(&b, "<CdtTrfTxInf>\r\n<PmtId>\r\n<InstrId>EMPTY</InstrId>\r\n<EndToEndId>EMPTY</EndToEndId>\r\n</PmtId>\r\n<Amt>\r\n<InstdAmt Ccy=\"IRR\">%d</InstdAmt>\r\n</Amt>\r\n<CdtrAgt>\r\n<FinInstnId>\r\n<BIC>EMPTY</BIC>\r\n</FinInstnId>\r\n</CdtrAgt>\r\n<Cdtr>\r\n<Nm>%s</Nm>\r\n<PstlAdr>\r\n<AdrLine>EMPTY</AdrLine>\r\n</PstlAdr>\r\n<Id>\r\n<PrvtId>\r\n<Othr>\r\n<Id>EMPTY-EMPTY</Id>\r\n</Othr>\r\n</PrvtId>\r\n</Id>\r\n</Cdtr>\r\n<CdtrAcct>\r\n<Id>\r\n<IBAN>%s</IBAN>\r\n</Id>\r\n</CdtrAcct>\r\n<RmtInf>\r\n<Ustrd>%s</Ustrd>\r\n</RmtInf>\r\n</CdtTrfTxInf>\r\n</PmtInf>\r\n", r.Amount, esc(r.Name), esc(r.IBAN), esc(r.Desc))
	}
	b.WriteString("</CstmrCdtTrfInitn>\r\n</Document>\r\n")
	return []byte(b.String())
}

type openFilename struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	TemplateName  *uint16
	ReservedPtr   uintptr
	ReservedInt   uint32
	FlagsEx       uint32
}

func dialog(save bool, extension string) string {
	buf := make([]uint16, 4096)
	if save {
		copy(buf, syscall.StringToUTF16(headers[0]+strings.Repeat("0", max(0, 9-len(headers[2])))+headers[2]+extension))
	}
	filterText := "فایل‌های " + extension + "\x00*" + extension + "\x00همه فایل‌ها\x00*.*\x00\x00"
	filterBuf := utf16.Encode([]rune(filterText))
	filter := &filterBuf[0]
	p := openFilename{Owner: hwnd, File: &buf[0], MaxFile: uint32(len(buf)), Filter: filter, Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST}
	p.StructSize = uint32(unsafe.Sizeof(p))
	if save {
		p.Flags |= OFN_OVERWRITEPROMPT
		p.DefExt = ptr(strings.TrimPrefix(extension, "."))
		if call(comdlg32.NewProc("GetSaveFileNameW"), uintptr(unsafe.Pointer(&p))) == 0 {
			return ""
		}
	} else {
		p.Flags |= OFN_FILEMUSTEXIST
		if call(comdlg32.NewProc("GetOpenFileNameW"), uintptr(unsafe.Pointer(&p))) == 0 {
			return ""
		}
	}
	runtime.KeepAlive(filterBuf)
	return uintStr(buf)
}
func load() {
	path := dialog(false, ".txt")
	if path == "" {
		return
	}
	if e := importTXT(path); e != nil {
		alert(e.Error(), true)
	} else {
		alert(fmt.Sprintf("%d ردیف بارگذاری شد.", len(rows)), false)
	}
}

// The original Mellat program names every bank submission as:
// <26-character origin IBAN><9-digit file sequence>.CCTI
// e.g. IR570120000000009874247639000000146.CCTI
func standardCCTIName() string {
	return headers[0] + strings.Repeat("0", 9-len(headers[2])) + headers[2] + ".CCTI"
}
func saveFile(ccti bool) {
	if e := validate(); e != nil {
		alert(e.Error(), true)
		return
	}
	ext := ".txt"
	if ccti {
		ext = ".CCTI"
	}
	selected := dialog(true, ext)
	if selected == "" {
		return
	}
	path := selected
	if ccti {
		// The save dialog is used only to choose a directory. Do not allow a
		// manually changed basename to break the bank's standard filename.
		path = filepath.Join(filepath.Dir(selected), standardCCTIName())
		if _, e := os.Stat(path); e == nil {
			if !ask("فایل خروجی با نام استاندارد از قبل موجود است:\n" + filepath.Base(path) + "\n\nفایل قبلی جایگزین شود؟") {
				return
			}
		} else if !os.IsNotExist(e) {
			alert("خطا هنگام بررسی فایل مقصد: "+e.Error(), true)
			return
		}
	}
	var b []byte
	var e error
	if ccti {
		b = exportCCTI()
	} else {
		var s string
		s, e = exportTXT()
		if e == nil {
			b, e = encodeCP1256(s)
		}
	}
	if e == nil {
		e = os.WriteFile(path, b, 0600)
	}
	if e != nil {
		alert(e.Error(), true)
		return
	}
	alert("فایل با موفقیت ذخیره شد:\n"+path, false)
}
func writeCrashLog(event string) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "MellatBatchFa")
	_ = os.MkdirAll(dir, 0700)
	f, e := os.OpenFile(filepath.Join(dir, "error.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  %s\r\n", time.Now().Format(time.RFC3339), event)
}

// About and help are actual RTL windows using the embedded Vazirmatn font.
// MessageBox uses Windows system UI font and does not reliably support RTL styling.
func infoWndProc(w, msg, wp, lp uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		if low(wp) == 990 && uint16(wp>>16) == 0 {
			call(user32.NewProc("DestroyWindow"), w)
			return 0
		}
	case WM_CLOSE:
		call(user32.NewProc("DestroyWindow"), w)
		return 0
	case WM_DESTROY:
		if w == helpDialog {
			helpDialog = 0
		}
		if w == aboutDialog {
			aboutDialog = 0
		}
		return 0
	}
	return call(defWindow, w, msg, wp, lp)
}
func ensureInfoWindowClass() bool {
	if infoClassRegistered {
		return true
	}
	inst := call(kernel32.NewProc("GetModuleHandleW"), 0)
	cls := "MellatBatchInfoV23"
	wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), WndProc: syscall.NewCallback(infoWndProc), Instance: inst, Bg: 6, Cursor: call(user32.NewProc("LoadCursorW"), 0, uintptr(32512)), Icon: call(user32.NewProc("LoadIconW"), inst, uintptr(1)), IconSmall: call(user32.NewProc("LoadIconW"), inst, uintptr(1)), Class: ptr(cls)}
	if call(user32.NewProc("RegisterClassExW"), uintptr(unsafe.Pointer(&wc))) == 0 {
		return false
	}
	infoClassRegistered = true
	return true
}
func showInfo(title, content string, target *uintptr, height int) {
	if *target != 0 {
		call(user32.NewProc("ShowWindow"), *target, SW_RESTORE)
		call(user32.NewProc("SetForegroundWindow"), *target)
		return
	}
	if !ensureInfoWindowClass() {
		alert("خطا در باز کردن پنجره اطلاعات.", true)
		return
	}
	inst := call(kernel32.NewProc("GetModuleHandleW"), 0)
	w := call(createWindow, uintptr(WS_EX_RTLREADING), uintptr(unsafe.Pointer(ptr("MellatBatchInfoV23"))), uintptr(unsafe.Pointer(ptr(title))), uintptr(WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE), 230, 110, 760, uintptr(height), hwnd, 0, inst, 0)
	if w == 0 {
		alert("پنجره اطلاعات باز نشد.", true)
		return
	}
	*target = w
	// The entire text is drawn by a native multi-line edit, not MessageBox.
	// ES_RIGHT controls alignment; WS_EX_RTLREADING controls reading order.
	e := call(createWindow, uintptr(WS_EX_RTLREADING|WS_EX_CLIENTEDGE), uintptr(unsafe.Pointer(ptr("EDIT"))), uintptr(unsafe.Pointer(ptr(content))), uintptr(WS_CHILD|WS_VISIBLE|WS_VSCROLL|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|ES_RIGHT), 20, 20, 700, uintptr(height-125), w, 991, inst, 0)
	call(sendMessage, e, WM_SETFONT, font, 1)
	btn := call(createWindow, uintptr(WS_EX_RTLREADING), uintptr(unsafe.Pointer(ptr("BUTTON"))), uintptr(unsafe.Pointer(ptr("بستن"))), uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_RIGHT), 310, uintptr(height-95), 120, 34, w, 990, inst, 0)
	call(sendMessage, btn, WM_SETFONT, font, 1)
	call(user32.NewProc("ShowWindow"), w, SW_SHOW)
	call(user32.NewProc("UpdateWindow"), w)
}
const personalUseDisclaimer = "این برنامه برای استفاده شخصی ایجاد شده و دارای تأییدیه امنیتی و عملکردی از بانک ملت نیست.\r\n" +
	"ضروری است استفاده‌کننده پیش از انجام هرگونه عملیات بانکی، اطلاعات و فایل‌های خروجی را شخصاً بررسی و تأیید کند.\r\n" +
	"مسئولیت هرگونه ایراد، اشتباه یا پیامد احتمالی ناشی از استفاده از برنامه بر عهده استفاده‌کننده است."

func showHelp() {
	helpText := "راهنمای استفاده از برنامه واریزهای پایا گروهی\r\n\r\n" +
		"۱. باز کردن: با انتخاب «باز کردن»، فایل TXT اطلاعات واریز را بارگذاری کنید.\r\n\r\n" +
		"۲. اطلاعات مبدأ: شماره شبای مبدأ، تاریخ شمسی، شماره فایل و نام صاحب حساب را بررسی کنید.\r\n\r\n" +
		"۳. افزودن ردیف: شماره شبای مقصد، نام دریافت‌کننده، مبلغ به ریال، علت انتقال و توضیحات را وارد کنید؛ سپس «افزودن ردیف» را بزنید.\r\n\r\n" +
		"۴. اصلاح ردیف: ردیف را در جدول انتخاب کنید، اطلاعات پایین فرم را تغییر دهید و «اصلاح ردیف» را بزنید.\r\n\r\n" +
		"۵. حذف ردیف: ردیف را انتخاب و «حذف ردیف» را بزنید.\r\n\r\n" +
		"۶. کنترل صحت شبا: ساختار و رقم کنترل شبای مبدأ و مقصدها بررسی می‌شود. این گزینه استعلام حساب یا نام صاحب حساب نیست.\r\n\r\n" +
		"۷. جمع کل: تعداد ردیف‌ها و جمع مبالغ به‌صورت خودکار محاسبه می‌شود.\r\n\r\n" +
		"۸. ذخیره اطلاعات: با «ذخیره اطلاعات» اطلاعات را به‌صورت TXT ذخیره کنید.\r\n\r\n" +
		"۹. ساخت خروجی برای بانک: فایل CCTI با نام استاندارد ساخته می‌شود؛ نام آن ترکیب شبای مبدأ و شماره فایل ۹ رقمی است.\r\n\r\n" +
		"سقف هر واریز: ۵۰۰٬۰۰۰٬۰۰۰ تومان (۵٬۰۰۰٬۰۰۰٬۰۰۰ ریال). مبالغ برنامه به ریال هستند.\r\n\r\n" +
		"کد منبع و به‌روزرسانی‌ها در GitHub:\r\nhttps://github.com/kariminia/Mellat-Paya-Batch-Transfer\r\n\r\n" +
		"\r\n\r\nاطلاعیه مسئولیت استفاده\r\n" + personalUseDisclaimer
	showInfo("راهنمای برنامه", helpText, &helpDialog, 725)
}
func showAbout() {
	aboutText := "برنامه غیر رسمی واریزهای پایا گروهی بانک ملت\r\n\r\n" +
		"نسخه: ۲.۵\r\n" +
		"تهیه‌شده توسط میثم کریمی‌نیا\r\n" +
		"مخزن کد منبع در GitHub:\r\nhttps://github.com/kariminia/Mellat-Paya-Batch-Transfer\r\n" +
		"سال ساخت: ۱۴۰۵ (۲۰۲۶ میلادی)\r\n" +
		"آخرین تغییر: ۱ آذر ۱۴۰۵ (۲۲ نوامبر ۲۰۲۶)\r\n\r\n" +
		"آخرین تغییرات:\r\n" +
		"• درج نشانی GitHub و اصلاح مشخصات تهیه‌کننده\r\n" +
		"• نمایش راهنما و درباره با فونت وزیرمتن و چینش راست‌به‌چپ\r\n" +
		"• افزایش سقف هر واریز به ۵۰۰ میلیون تومان معادل ۵ میلیارد ریال\r\n" +
		"• حفظ قالب TXT و ساختار CCTI و نام‌گذاری استاندارد خروجی\r\n" +
		"• حذف کلیدهای ماه قبل و ماه بعد از تقویم شمسی\r\n\r\n" +
		"اطلاعیه مسئولیت استفاده\r\n" + personalUseDisclaimer
	showInfo("درباره برنامه", aboutText, &aboutDialog, 650)
}

type paintStruct struct {
	DC                       uintptr
	Erase                    int32
	Left, Top, Right, Bottom int32
	Restore                  int32
	IncUpdate                int32
	Reserved                 [32]byte
}
type rect struct{ Left, Top, Right, Bottom int32 }

func captionChild(h uintptr) uintptr {
	if captionChildren == nil {
		captionChildren = make(map[uintptr]bool)
	}
	captionChildren[h] = true
	return h
}
func wndProc(w, msg, wp, lp uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			writeCrashLog(fmt.Sprintf("Recovered window callback panic (msg=%d, wparam=%d): %v\n%s", msg, wp, r, debug.Stack()))
			alert("خطای داخلی ثبت شد. فایل error.log در پوشه MellatBatchFa داخل AppData\\Local را ارسال کنید.", true)
			ret = 0
		}
	}()
	switch msg {
	case WM_CREATE:
		return 0
	case WM_CTLCOLORSTATIC:
		if w == hwnd && captionChildren[lp] {
			// Use exactly the same brush and color for caption text, icons and empty strip.
			call(gdi32.NewProc("SetBkColor"), wp, 0x00F0F0F0)
			call(gdi32.NewProc("SetTextColor"), wp, 0x00252525)
			return captionBrush
		}
	case WM_PAINT:
		if w == hwnd {
			var ps paintStruct
			dc := call(user32.NewProc("BeginPaint"), hwnd, uintptr(unsafe.Pointer(&ps)))
			if dc != 0 {
				var client rect
				call(user32.NewProc("GetClientRect"), hwnd, uintptr(unsafe.Pointer(&client)))
				strip := rect{Left: 0, Top: 0, Right: client.Right, Bottom: 52}
				call(user32.NewProc("FillRect"), dc, uintptr(unsafe.Pointer(&strip)), captionBrush)
				call(user32.NewProc("EndPaint"), hwnd, uintptr(unsafe.Pointer(&ps)))
			}
			return 0
		}
	case WM_NCHITTEST:
		hit := call(defWindow, w, msg, wp, lp)
		// Drag from the empty portion of the custom title strip. Native resize borders
		// take precedence, and the three caption buttons remain independent children.
		if w == hwnd && hit == HTCLIENT {
			var r struct{ Left, Top, Right, Bottom int32 }
			call(user32.NewProc("GetWindowRect"), hwnd, uintptr(unsafe.Pointer(&r)))
			sy := int(int16(uint16(lp >> 16)))
			sx := int(int16(uint16(lp)))
			if sy-int(r.Top) >= 6 && sy-int(r.Top) < 43 && sx-int(r.Left) > 282 {
				return HTCAPTION
			}
		}
		return hit
	case WM_COMMAND:
		id := low(wp)
		notification := int(uint16(wp >> 16))
		if id >= 101 && id <= 110 && notification == 0 {
			writeCrashLog(fmt.Sprintf("Button clicked: %d", id))
		}
		// Only button BN_CLICKED (0) or listbox LBN_SELCHANGE (1) trigger actions.
		if id == 201 {
			return 0
		}
		if id >= 400 && id <= 430 && notification == 0 {
			choosePickerDay(id - 399)
			return 0
		}
		if notification == 0 {
			switch id {
			case 108:
				openPicker()
				return 0
			case 321:
				pickerYear--
				if pickerYear < 1405 {
					pickerYear = 1405
				}
				drawPicker()
				return 0
			case 322:
				pickerYear++
				if pickerYear > 1430 {
					pickerYear = 1430
				}
				drawPicker()
				return 0
			}
		}
		if (id == 325 || id == 326) && notification == 1 {
			pickerSelectYearMonth()
			return 0
		}
		if notification != 0 {
			return 0
		}
		if w == hwnd {
			switch id {
			case 901:
				call(user32.NewProc("DestroyWindow"), hwnd)
				return 0
			case 902:
				call(user32.NewProc("ShowWindow"), hwnd, SW_MINIMIZE)
				return 0
			case 903:
				if call(user32.NewProc("IsZoomed"), hwnd) != 0 {
					call(user32.NewProc("ShowWindow"), hwnd, SW_RESTORE)
				} else {
					call(user32.NewProc("ShowWindow"), hwnd, SW_MAXIMIZE)
				}
				return 0
			}
		}
		switch id {
		case 101:
			load()
		case 102:
			addOrUpdate(false)
		case 103:
			addOrUpdate(true)
		case 104:
			deleteSelected()
		case 105:
			saveFile(false)
		case 106:
			saveFile(true)
		case 107:
			checkAccounts()
		case 109:
			showAbout()
		case 110:
			showHelp()
		}
		return 0
	case WM_NOTIFY:
		if lp != 0 {
			n := (*nmHeader)(unsafe.Pointer(lp))
			if n.IDFrom == 201 && n.Code == -101 {
				onSelect()
				return 0
			}
		}
	case WM_DESTROY:
		if w == hwnd {
			call(user32.NewProc("PostQuitMessage"), 0)
		} else if w == picker {
			picker = 0
		}
		return 0
	}
	return call(defWindow, w, msg, wp, lp)
}

type wndClass struct {
	Size      uint32
	Style     uint32
	WndProc   uintptr
	ClsExtra  int32
	WndExtra  int32
	Instance  uintptr
	Icon      uintptr
	Cursor    uintptr
	Bg        uintptr
	Menu      *uint16
	Class     *uint16
	IconSmall uintptr
}
type point struct{ X, Y int32 }
type msg struct {
	Hwnd           uintptr
	Message        uint32
	Wparam, Lparam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Register SysListView32 common control class.
	initCommon := struct {
		Size uint32
		ICC  uint32
	}{Size: 8, ICC: 1}
	call(syscall.NewLazyDLL("comctl32.dll").NewProc("InitCommonControlsEx"), uintptr(unsafe.Pointer(&initCommon)))
	inst := call(kernel32.NewProc("GetModuleHandleW"), 0)
	if !loadEmbeddedFont() {
		writeCrashLog("Embedded Vazirmatn font failed to load")
	}
	font = call(gdi32.NewProc("CreateFontW"), uintptr(21), 0, 0, 0, uintptr(400), 0, 0, 0, uintptr(1), 0, 0, 0, 0, uintptr(unsafe.Pointer(ptr("Vazirmatn"))))
	cls := "MellatBatchNativeFa"
	wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), WndProc: syscall.NewCallback(wndProc), Instance: inst, Bg: uintptr(6), Cursor: call(user32.NewProc("LoadCursorW"), 0, uintptr(32512)), Icon: call(user32.NewProc("LoadIconW"), inst, uintptr(1)), IconSmall: call(user32.NewProc("LoadIconW"), inst, uintptr(1)), Class: ptr(cls)}
	if call(user32.NewProc("RegisterClassExW"), uintptr(unsafe.Pointer(&wc))) == 0 {
		alert("خطای ساخت پنجره", true)
		return
	}
	// IMPORTANT: Never mirror the entire top-level Win32 window. WS_EX_LAYOUTRTL
	// at this level mirrored child coordinates and broke the form in v1.3/v1.4.
	// A custom caption gives left-side buttons without changing coordinate space.
	hwnd = call(createWindow, 0, uintptr(unsafe.Pointer(ptr(cls))), uintptr(unsafe.Pointer(ptr("برنامه غیر رسمی واریزهای پایا گروهی بانک ملت"))), uintptr(WS_POPUP|WS_THICKFRAME|WS_SYSMENU|WS_MINIMIZEBOX|WS_MAXIMIZEBOX|WS_VISIBLE), 90, 60, 1140, 865, 0, 0, inst, 0)
	if hwnd == 0 {
		writeCrashLog("Unable to create main window")
		return
	}
	captionBrush = call(gdi32.NewProc("CreateSolidBrush"), 0x00F0F0F0)
	captionChildren = make(map[uintptr]bool)
	// Left window controls; the main title and icon are on the right.
	// Borderless caption controls: clickable STATIC glyphs instead of 3D BUTTONs.
	// SS_NOTIFY sends STN_CLICKED (notification code 0) to WM_COMMAND.
	captionFont := call(gdi32.NewProc("CreateFontW"), uintptr(29), 0, 0, 0, uintptr(700), 0, 0, 0, uintptr(1), 0, 0, 0, 0, uintptr(unsafe.Pointer(ptr("Segoe UI Symbol"))))
	for _, c := range []struct {
		glyph string
		id, x int
	}{{"×", 901, 12}, {"□", 903, 60}, {"−", 902, 108}} {
		h := captionChild(window("STATIC", c.glyph, WS_CHILD|WS_VISIBLE|0x00000100|0x00000001, c.x, 7, 44, 35, c.id, hwnd)) // SS_NOTIFY|SS_CENTER
		if captionFont != 0 {
			call(sendMessage, h, WM_SETFONT, captionFont, 1)
		}
	}
	// About link beside the caption icons; text only, no button frame.
	aboutLink := captionChild(window("STATIC", "درباره ما", WS_CHILD|WS_VISIBLE|0x00000100|0x00000002, 158, 10, 100, 30, 109, hwnd)) // SS_NOTIFY|SS_RIGHT
	call(sendMessage, aboutLink, WM_SETFONT, font, 1)
	helpLink := captionChild(window("STATIC", "راهنما", WS_CHILD|WS_VISIBLE|0x00000100|0x00000002, 262, 10, 82, 30, 110, hwnd)) // SS_NOTIFY|SS_RIGHT
	call(sendMessage, helpLink, WM_SETFONT, font, 1)
	mainIcon = captionChild(ctl("STATIC", "", 1080, 10, 28, 28, 0, 0x00000003)) // SS_ICON
	if hicon := call(user32.NewProc("LoadIconW"), inst, uintptr(1)); hicon != 0 {
		call(sendMessage, mainIcon, 0x0170, hicon, 0)
	}
	// Prominent right-aligned Persian title. Keep the disclaimer in About.
	titleFont = call(gdi32.NewProc("CreateFontW"), uintptr(31), 0, 0, 0, uintptr(700), 0, 0, 0, uintptr(1), 0, 0, 0, 0, uintptr(unsafe.Pointer(ptr("Vazirmatn"))))
	titleH := captionChild(ctl("STATIC", "برنامه غیر رسمی واریزهای پایا گروهی بانک ملت", 350, 9, 722, 38, 0, 0))
	if titleFont != 0 {
		call(sendMessage, titleH, WM_SETFONT, titleFont, 1)
	}
	topButtons := []struct {
		Text string
		ID   int
		X, W int
	}{{"باز کردن", 101, 960, 156}, {"ذخیره اطلاعات", 105, 792, 156}, {"ساخت خروجی برای بانک", 106, 585, 195}}
	for _, x := range topButtons {
		ctl("BUTTON", x.Text, x.X, 85, x.W, 36, x.ID, WS_TABSTOP)
	}
	names := []string{"شبا مبدأ", "تاریخ واریز (شمسی)", "شماره فایل", "نام صاحب حساب"}
	xs := []int{65, 380, 640, 825}
	ws := []int{290, 230, 170, 280}
	for i := range fields {
		ctl("STATIC", names[i], xs[i], 129, ws[i], 22, 0, 0)
		style := uint32(WS_BORDER | ES_LEFT | ES_AUTOHSCROLL | WS_TABSTOP)
		if i == 1 {
			style |= ES_READONLY
		}
		fieldW := ws[i]
		if i == 1 {
			fieldW -= 39
		}
		fields[i] = ctl("EDIT", "", xs[i], 152, fieldW, 27, 0, style)
		v := headers[[]int{0, 1, 2, 5}[i]]
		if i == 1 {
			v = formattedDate(v)
		}
		set(fields[i], v)
		if i == 1 {
			ctl("BUTTON", "▾", xs[i]+fieldW+3, 152, 34, 27, 108, WS_TABSTOP)
		}
	}
	ctl("STATIC", "فهرست واریزها (برای ویرایش، روی ردیف کلیک کنید)", 755, 190, 355, 24, 0, 0)
	hlist = call(createWindow, uintptr(WS_EX_LAYOUTRTL|WS_EX_RTLREADING|WS_EX_NOINHERITLAYOUT), uintptr(unsafe.Pointer(ptr("SysListView32"))), uintptr(unsafe.Pointer(ptr(""))), uintptr(WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS), uintptr(18), uintptr(218), uintptr(1090), uintptr(300), hwnd, uintptr(201), inst, 0)
	call(sendMessage, hlist, WM_SETFONT, font, 1)
	initTable()
	labels := []string{"شماره شبا مقصد", "نام دریافت‌کننده", "مبلغ (ریال)", "علت انتقال", "شرح واریز"}
	x := []int{390, 825, 650, 205, 18}
	widths := []int{250, 280, 160, 170, 180}
	for i := range entries {
		ctl("STATIC", labels[i], x[i], 578, widths[i], 22, 0, 0)
		if i == 3 {
			entries[i] = ctl("COMBOBOX", "", x[i], 603, widths[i], 270, 210, WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST)
		} else {
			style := uint32(ES_RIGHT)
			if i == 0 || i == 2 {
				style = ES_LEFT
			}
			entries[i] = ctl("EDIT", "", x[i], 603, widths[i], 28, 0, WS_BORDER|style|ES_AUTOHSCROLL|WS_TABSTOP)
		}
	}
	// Numerical values are intentionally shown without unverified banking descriptions.
	for code := 1; code <= 19; code++ {
		ensureCode(strconv.Itoa(code))
	}
	selectCode("1")
	rowButtons := []struct {
		Text string
		ID   int
		X, W int
	}{{"افزودن ردیف", 102, 970, 138}, {"اصلاح ردیف", 103, 824, 138}, {"حذف ردیف", 104, 678, 138}, {"کنترل صحت شبا", 107, 505, 165}}
	for _, x := range rowButtons {
		ctl("BUTTON", x.Text, x.X, 643, x.W, 36, x.ID, WS_TABSTOP)
	}
	ctl("STATIC", "ابتدا یک ردیف را انتخاب کنید یا اطلاعات ردیف جدید را وارد نمایید.", 175, 688, 930, 27, 0, 0)
	summary = ctl("STATIC", "", 20, 742, 1088, 40, 0, 0)
	// Keep the bottom area uncluttered, matching the supplied reference screenshot.
	refresh()
	call(user32.NewProc("ShowWindow"), hwnd, SW_SHOW)
	call(user32.NewProc("UpdateWindow"), hwnd)
	var m msg
	for {
		status := call(user32.NewProc("GetMessageW"), uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if status == 0 || status == ^uintptr(0) {
			break
		}
		call(user32.NewProc("TranslateMessage"), uintptr(unsafe.Pointer(&m)))
		call(user32.NewProc("DispatchMessageW"), uintptr(unsafe.Pointer(&m)))
	}
	if captionBrush != 0 {
		call(gdi32.NewProc("DeleteObject"), captionBrush)
	}
	if captionFont != 0 {
		call(gdi32.NewProc("DeleteObject"), captionFont)
	}
	if titleFont != 0 {
		call(gdi32.NewProc("DeleteObject"), titleFont)
	}
	if font != 0 {
		call(gdi32.NewProc("DeleteObject"), font)
	}
	if embeddedFontHandle != 0 {
		call(gdi32.NewProc("RemoveFontMemResourceEx"), embeddedFontHandle)
	}
	runtime.KeepAlive(embeddedVazirmatn)
}
