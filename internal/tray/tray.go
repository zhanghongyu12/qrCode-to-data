// Package tray 提供 Windows 系统托盘图标：常驻右下角，菜单可「打开桌面端」或「退出」。
package tray

import (
	"bytes"
	"encoding/binary"

	"github.com/getlantern/systray"
)

// Handlers 托盘回调。
type Handlers struct {
	OnOpen func() // 点击「打开桌面端」
	OnQuit func() // 点击「退出」（systray 退出后回调）
}

// Run 运行系统托盘，阻塞直到用户点击「退出」。
// letter 为托盘图标中心显示的大写字母（如 "A"/"B"/"Q"），用于区分不同端。
func Run(letter string, h Handlers) {
	systray.Run(func() {
		systray.SetTitle("qrcd")
		systray.SetTooltip("qrcd")
		systray.SetIcon(iconICO(letter))
		mOpen := systray.AddMenuItem("打开桌面端", "打开窗口")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "退出 qrcd")
		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					if h.OnOpen != nil {
						h.OnOpen()
					}
				case <-mQuit.ClickedCh:
					systray.Quit()
				}
			}
		}()
	}, func() {
		if h.OnQuit != nil {
			h.OnQuit()
		}
	})
}

// glyphs 5×7 点阵大写字母（1=实心，0=透明），绘制托盘图标中心的白色字母。
var glyphs = map[string][7]string{
	"A": {
		"01110",
		"10001",
		"10001",
		"11111",
		"10001",
		"10001",
		"10001",
	},
	"B": {
		"11110",
		"10001",
		"10001",
		"11110",
		"10001",
		"10001",
		"11110",
	},
	"Q": {
		"01110",
		"10001",
		"10001",
		"10001",
		"10001",
		"10011",
		"01101",
	},
}

// iconICO 生成 32×32 32bpp 的托盘图标（.ico 文件内容）：
// 绿色圆角方块 + 居中的白色大写字母（5×7 点阵 3× 放大 → 15×21）。
func iconICO(letter string) []byte {
	const size = 32
	const scale = 3
	const gw, gh = 5, 7
	lx := (size - gw*scale) / 2 // 字母左偏移（水平居中）
	ly := (size - gh*scale) / 2 // 字母上偏移（垂直居中）

	g, ok := glyphs[letter]
	if !ok {
		g = glyphs["Q"]
	}

	green := [4]byte{0x77, 0xaa, 0x22, 0xff} // #22aa77（BGRA）
	white := [4]byte{0xff, 0xff, 0xff, 0xff}
	transparent := [4]byte{0, 0, 0, 0}

	pix := func(x, y int) [4]byte {
		// 绿色圆角方块（边距 3，四角 2px 圆角）
		if x >= 3 && x < 29 && y >= 3 && y < 29 && !((x < 5 || x >= 27) && (y < 5 || y >= 27)) {
			gx := (x - lx) / scale
			gy := (y - ly) / scale
			if gx >= 0 && gx < gw && gy >= 0 && gy < gh && g[gy][gx] == '1' {
				return white
			}
			return green
		}
		return transparent
	}

	// XOR 位图：bottom-up，每像素 BGRA 4 字节
	xor := make([]byte, 0, size*size*4)
	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			c := pix(x, y)
			xor = append(xor, c[0], c[1], c[2], c[3])
		}
	}
	andMask := make([]byte, size*4) // 32×4=128 字节，全 0 = 不透明

	// BITMAPINFOHEADER（40 字节）
	bih := make([]byte, 40)
	binary.LittleEndian.PutUint32(bih[0:], 40)     // biSize
	binary.LittleEndian.PutUint32(bih[4:], size)   // biWidth
	binary.LittleEndian.PutUint32(bih[8:], size*2) // biHeight（XOR+AND）
	binary.LittleEndian.PutUint16(bih[12:], 1)     // biPlanes
	binary.LittleEndian.PutUint16(bih[14:], 32)    // biBitCount

	imageData := append(bih, xor...)
	imageData = append(imageData, andMask...)

	// ICONDIR（6 字节）+ ICONDIRENTRY（16 字节）
	dir := make([]byte, 6)
	binary.LittleEndian.PutUint16(dir[2:], 1) // type=icon
	binary.LittleEndian.PutUint16(dir[4:], 1) // count=1

	entry := make([]byte, 16)
	entry[0] = size
	entry[1] = size
	binary.LittleEndian.PutUint16(entry[4:], 1)                     // planes
	binary.LittleEndian.PutUint16(entry[6:], 32)                    // bitCount
	binary.LittleEndian.PutUint32(entry[8:], uint32(len(imageData))) // size
	binary.LittleEndian.PutUint32(entry[12:], 22)                   // offset（6+16）

	var buf bytes.Buffer
	buf.Write(dir)
	buf.Write(entry)
	buf.Write(imageData)
	return buf.Bytes()
}
