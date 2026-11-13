// Command icongen draws the app icon procedurally (the same lemon slice as
// the game's background) and writes it for iOS and Android.
//
//	go run ./tools/icongen
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func icon(n int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	const ss = 3
	fruit := [][3]float64{{0.96, 0.83, 0.16}, {0.49, 0.80, 0.28}, {0.94, 0.38, 0.37}, {0.37, 0.53, 0.92}}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var acc [3]float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					u := (float64(x) + (float64(sx)+0.5)/ss) / float64(n)
					v := (float64(y) + (float64(sy)+0.5)/ss) / float64(n)
					c := [3]float64{lerp(0.106, 0.043, v), lerp(0.082, 0.051, v), lerp(0.188, 0.086, v)}
					// lemon slice
					du, dv := (u-0.5)/0.36, (v-0.43)/0.36
					d := math.Hypot(du, dv)
					if d < 1 {
						l := [3]float64{1, 0.85 * (0.86 + 0.14*d), 0.29}
						switch {
						case d > 0.9:
							l = [3]float64{0.95, 0.76, 0}
						case d > 0.8 || d < 0.1:
							l = [3]float64{1, 0.95, 0.77}
						default:
							a := (math.Atan2(dv, du) + math.Pi) / (2 * math.Pi) * 10
							f := a - math.Floor(a)
							if math.Min(f, 1-f)*2*math.Pi/10*d < 0.04 {
								l = [3]float64{1, 0.95, 0.77}
							}
						}
						c = l
					}
					// a heap of sand along the bottom, in the four colours
					h := 0.80 + 0.07*math.Cos((u-0.5)*3.4)
					if v > h {
						gx, gy := int(u*28), int(v*28)
						k := fruit[(gx/5+gy/3+gx*gy/17)%4]
						sh := 0.82 + 0.18*float64((gx*7+gy*13)%5)/4
						c = [3]float64{k[0] * sh, k[1] * sh, k[2] * sh}
					}
					for i := range acc {
						acc[i] += c[i]
					}
				}
			}
			to := func(v float64) uint8 { return uint8(math.Max(0, math.Min(1, v/ss/ss))*255 + 0.5) }
			img.SetRGBA(x, y, color.RGBA{to(acc[0]), to(acc[1]), to(acc[2]), 255})
		}
	}
	return img
}

func write(path string, img image.Image) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

func main() {
	write("ios/LemonDrop/Assets.xcassets/AppIcon.appiconset/icon-1024.png", icon(1024))
	for dir, n := range map[string]int{"mdpi": 48, "hdpi": 72, "xhdpi": 96, "xxhdpi": 144, "xxxhdpi": 192} {
		write("android/app/src/main/res/mipmap-"+dir+"/ic_launcher.png", icon(n))
	}
}
