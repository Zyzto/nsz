package main

import (
	"context"
	"image"
	"image/color"
	"math"
	"math/rand"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// Galaxy background: heavy effects are drawn at a capped internal resolution, then
// nearest-neighbor upscaled to the window. Fyne draws the final image as a GPU
// texture, so enlargement uses hardware filtering and the CPU avoids filling
// every physical pixel with the expensive passes (black holes, aurora).

type galaxy struct {
	raster *canvas.Raster
	mu     sync.Mutex

	phase      float64
	stars      []star
	meteors    []meteor
	blackHoles []blackHole
	plants     []plantLite
	comet      cometState

	sceneReady bool
	rng        *rand.Rand

	// Last window size (for meteor physics & coordinate mapping).
	lastW, lastH int

	// Reused full-size output buffer (reduces GC during resize / animation).
	fullImg      *image.RGBA
	fullW, fullH int

	// Reused internal buffer.
	smallImg       *image.RGBA
	smallW, smallH int
}

type star struct {
	x, y, twinkle float64
	brightness    float64
}

type meteor struct {
	x, y, vx, vy, life float64
	len                float64
}

type blackHole struct {
	nx, ny   float64
	rFrac    float64
	spinOffs float64
}

type cometState struct {
	angle      float64
	dist       float64
	tailLen    float64
	brightness float64
}

// plantLite: cheap silhouettes at bottom; kinds 0=grass, 1=mound, 2=frond
type plantLite struct {
	kind int
	nx   float64
	seed float64
}

const (
	maxMeteors        = 12
	meteorSpawnChance = 0.048
	fixedSceneSeed    = 0x4E53475A4247 // stable starfield / holes across resizes
	starCount         = 520
	internalMaxEdge   = 640 // target cap for longest side of internal buffer (tune for perf)
	auroraStep        = 2   // sample every Nth column on internal buffer
	maxPlants         = 48
)

func newGalaxy(ctx context.Context) *galaxy {
	g := &galaxy{}
	g.raster = canvas.NewRaster(func(w, h int) image.Image {
		return g.render(w, h)
	})
	g.raster.SetMinSize(fyne.NewSize(32, 32))

	go func() {
		tick := time.NewTicker(time.Second / 28)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				canvas.Refresh(g.raster)
			}
		}
	}()
	return g
}

func (g *galaxy) object() fyne.CanvasObject {
	return g.raster
}

func internalSize(ww, wh int) (iw, ih int) {
	if ww <= 0 || wh <= 0 {
		return 1, 1
	}
	maxE := ww
	if wh > maxE {
		maxE = wh
	}
	// ceil(maxE / internalMaxEdge); >= 1 so we never divide by zero
	scale := (maxE + internalMaxEdge - 1) / internalMaxEdge
	if scale < 1 {
		scale = 1
	}
	iw = max(1, ww/scale)
	ih = max(1, wh/scale)
	return iw, ih
}

func (g *galaxy) render(ww, wh int) image.Image {
	if ww <= 0 || wh <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	g.mu.Lock()
	g.phase += 0.05
	if !g.sceneReady {
		g.regenScene()
		g.sceneReady = true
	}
	g.lastW, g.lastH = ww, wh
	g.updateMeteors(ww, wh)
	g.updateComet(ww, wh)

	phase := g.phase
	stars := g.stars
	meteors := append([]meteor(nil), g.meteors...)
	holes := g.blackHoles
	plants := g.plants
	comet := g.comet
	g.mu.Unlock()

	iw, ih := internalSize(ww, wh)

	if g.smallImg == nil || g.smallW != iw || g.smallH != ih {
		g.smallImg = image.NewRGBA(image.Rect(0, 0, iw, ih))
		g.smallW, g.smallH = iw, ih
	}
	small := g.smallImg

	drawSkyGradient(small, ih)

	orbs := []struct {
		cx, cy, r  float64
		cr, cg, cb uint8
		strength   float64
	}{
		{0.22 + 0.04*math.Sin(phase*0.7), 0.32 + 0.03*math.Cos(phase*0.5), 0.4, 160, 50, 130, 0.32},
		{0.74 + 0.03*math.Cos(phase*0.65), 0.26 + 0.04*math.Sin(phase*0.85), 0.36, 70, 35, 190, 0.3},
		{0.5 + 0.028*math.Sin(phase*1.05), 0.58 + 0.022*math.Cos(phase*1.12), 0.42, 180, 90, 50, 0.28},
		{0.12 + 0.025*math.Cos(phase*0.95), 0.68 + 0.035*math.Sin(phase*0.45), 0.3, 30, 100, 180, 0.26},
		{0.88 + 0.02*math.Sin(phase*0.55), 0.72 + 0.02*math.Cos(phase*0.75), 0.25, 120, 40, 160, 0.22},
	}
	md := minDim(iw, ih)
	for _, o := range orbs {
		cx := int(o.cx * float64(iw))
		cy := int(o.cy * float64(ih))
		r := int(o.r * float64(md))
		paintOrb(small, cx, cy, r, o.cr, o.cg, o.cb, o.strength)
	}

	for _, bh := range holes {
		cx := int(bh.nx * float64(iw))
		cy := int(bh.ny * float64(ih))
		r := int(bh.rFrac * float64(md))
		if r > 4 {
			paintBlackHole(small, cx, cy, r, phase+bh.spinOffs)
		}
	}

	for _, s := range stars {
		tw := 0.55 + 0.45*math.Sin(phase*1.4+s.twinkle)
		t := tw * s.brightness * 0.92
		if t > 1 {
			t = 1
		}
		x := int(s.x * float64(iw))
		y := int(s.y * float64(ih))
		if x >= 0 && x < iw && y >= 0 && y < ih {
			base := small.RGBAAt(x, y)
			small.Set(x, y, color.RGBA{
				R: lerpU8(base.R, 248, t),
				G: lerpU8(base.G, 252, t),
				B: lerpU8(base.B, 255, t),
				A: 255,
			})
		}
	}

	drawAuroraSparse(small, iw, ih, phase)

	for _, m := range meteors {
		if m.life <= 0 {
			continue
		}
		mx := m.x * float64(iw) / float64(ww)
		my := m.y * float64(ih) / float64(wh)
		paintMeteor(small, iw, ih, meteor{x: mx, y: my, vx: m.vx, vy: m.vy, life: m.life, len: m.len * float64(ih) / float64(wh)})
	}

	drawCometScaled(small, iw, ih, comet, phase)

	drawPlantsLite(small, iw, ih, phase, plants)

	// Upscale internal buffer → full window (CPU copy; display scales on GPU).
	if g.fullImg == nil || g.fullW != ww || g.fullH != wh {
		g.fullImg = image.NewRGBA(image.Rect(0, 0, ww, wh))
		g.fullW, g.fullH = ww, wh
	}
	nearestUpscale(small, g.fullImg, iw, ih, ww, wh)
	return g.fullImg
}

func nearestUpscale(src *image.RGBA, dst *image.RGBA, sw, sh, dw, dh int) {
	for y := 0; y < dh; y++ {
		sy := (y * sh) / dh
		if sy >= sh {
			sy = sh - 1
		}
		for x := 0; x < dw; x++ {
			sx := (x * sw) / dw
			if sx >= sw {
				sx = sw - 1
			}
			dst.Set(x, y, src.RGBAAt(sx, sy))
		}
	}
}

func (g *galaxy) regenScene() {
	g.rng = rand.New(rand.NewSource(fixedSceneSeed))

	g.stars = make([]star, starCount)
	for i := range g.stars {
		g.stars[i] = star{
			x:          g.rng.Float64(),
			y:          g.rng.Float64() * 0.92,
			twinkle:    g.rng.Float64() * math.Pi * 2,
			brightness: 0.2 + g.rng.Float64()*0.8,
		}
	}

	nh := 2 + g.rng.Intn(2)
	g.blackHoles = make([]blackHole, nh)
	for i := range g.blackHoles {
		g.blackHoles[i] = blackHole{
			nx:       0.15 + g.rng.Float64()*0.7,
			ny:       0.12 + g.rng.Float64()*0.55,
			rFrac:    0.06 + g.rng.Float64()*0.05,
			spinOffs: g.rng.Float64() * math.Pi * 2,
		}
	}

	nP := maxPlants
	g.plants = make([]plantLite, nP)
	for i := range g.plants {
		g.plants[i] = plantLite{
			kind: g.rng.Intn(3),
			nx:   (float64(i)+0.5)/float64(nP) + (g.rng.Float64()-0.5)*0.04,
			seed: g.rng.Float64() * math.Pi * 2,
		}
	}

	g.meteors = g.meteors[:0]
	g.comet = cometState{
		angle:      g.rng.Float64() * math.Pi * 2,
		dist:       0.35 + g.rng.Float64()*0.25,
		tailLen:    0.12 + g.rng.Float64()*0.08,
		brightness: 0.55 + g.rng.Float64()*0.35,
	}
}

func (g *galaxy) updateMeteors(w, h int) {
	sc := float64(minDim(w, h)) / 520.0
	if sc < 0.35 {
		sc = 0.35
	}
	if sc > 1.8 {
		sc = 1.8
	}

	alive := g.meteors[:0]
	for _, m := range g.meteors {
		m.x += m.vx * sc
		m.y += m.vy * sc
		m.life -= 0.017
		if m.life > 0 && m.x > -80 && m.x < float64(w)+80 && m.y > -80 && m.y < float64(h)+80 {
			alive = append(alive, m)
		}
	}
	g.meteors = alive

	if len(g.meteors) < maxMeteors && g.rng.Float64() < meteorSpawnChance {
		g.spawnMeteor(w, h)
	}
}

func (g *galaxy) spawnMeteor(w, h int) {
	edge := g.rng.Intn(4)
	var x, y float64
	switch edge {
	case 0:
		x, y = g.rng.Float64()*float64(w), -20
	case 1:
		x, y = -20, g.rng.Float64()*float64(h)*0.6
	case 2:
		x, y = float64(w)+20, g.rng.Float64()*float64(h)*0.5
	default:
		x, y = -g.rng.Float64()*100, g.rng.Float64()*float64(h)*0.35
	}
	ang := g.rng.Float64()*0.5 + 0.35
	if g.rng.Float64() < 0.5 {
		ang = math.Pi/4 + g.rng.Float64()*0.4
	}
	v := 7 + g.rng.Float64()*11
	g.meteors = append(g.meteors, meteor{
		x: x, y: y,
		vx:   math.Cos(ang) * v,
		vy:   math.Sin(ang) * v,
		life: 0.85 + g.rng.Float64()*0.45,
		len:  35 + g.rng.Float64()*90,
	})
}

func (g *galaxy) updateComet(w, h int) {
	_ = w
	_ = h
	g.comet.angle += 0.004 + 0.001*math.Sin(g.phase*0.2)
	g.comet.dist = 0.32 + 0.08*math.Sin(g.phase*0.15)
}

func drawSkyGradient(img *image.RGBA, h int) {
	w := img.Rect.Dx()
	for y := 0; y < h; y++ {
		v := float64(y) / float64(h)
		br := uint8(4 + v*20)
		bg := uint8(3 + v*14)
		bb := uint8(18 + v*52)
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: br, G: bg, B: bb, A: 255})
		}
	}
}

func drawAuroraSparse(img *image.RGBA, w, h int, phase float64) {
	top := int(float64(h) * 0.42)
	for y := 0; y < top; y++ {
		v := float64(y) / float64(h)
		for x := 0; x < w; x += auroraStep {
			u := float64(x) / float64(w)
			wave := math.Sin(u*8+phase*1.1) * math.Cos(v*6+phase*0.7)
			if wave < 0.55 {
				continue
			}
			t := (wave - 0.55) * 0.45
			if t <= 0 {
				continue
			}
			ar := uint8(40 + 80*wave)
			ag := uint8(200 + 55*math.Sin(phase+u*4))
			ab := uint8(140 + 80*math.Cos(phase*0.9+v*5))
			for dx := 0; dx < auroraStep && x+dx < w; dx++ {
				base := img.RGBAAt(x+dx, y)
				img.Set(x+dx, y, color.RGBA{
					R: lerpU8(base.R, ar, t*0.22),
					G: lerpU8(base.G, ag, t*0.28),
					B: lerpU8(base.B, ab, t*0.26),
					A: 255,
				})
			}
		}
	}
}

func drawPlantsLite(img *image.RGBA, w, h int, phase float64, plants []plantLite) {
	floor := float64(h) * 0.99
	zoneTop := int(floor - float64(h)*0.18)
	if zoneTop < 0 {
		zoneTop = 0
	}

	for _, p := range plants {
		bx := p.nx * float64(w)
		s := math.Sin(phase*1.6+p.seed) * 4

		switch p.kind {
		case 0: // grass: thin vertical strokes
			stemH := float64(h) * (0.06 + 0.04*math.Abs(math.Sin(p.seed)))
			for b := 0; b < 4; b++ {
				px := int(bx + float64(b-2)*2 + s)
				if px < 0 || px >= w {
					continue
				}
				for yy := int(floor - stemH); yy < int(floor); yy++ {
					if yy >= zoneTop && yy < h {
						base := img.RGBAAt(px, yy)
						img.Set(px, yy, color.RGBA{
							R: lerpU8(base.R, 12, 0.55),
							G: lerpU8(base.G, 55, 0.6),
							B: lerpU8(base.B, 42, 0.55),
							A: 255,
						})
					}
				}
			}
		case 1: // mound: single soft blob
			cy := int(floor - float64(h)*0.04)
			cx := int(bx + s)
			r := int(float64(w) / float64(maxPlants) * 1.2)
			if r < 4 {
				r = 4
			}
			paintOrb(img, cx, cy, r, 20, 70, 50, 0.45)
		case 2: // frond: short arc of dots
			ang := -math.Pi/2 + math.Sin(phase+p.seed)*0.4
			L := float64(h) * 0.07
			for k := 0; k < 6; k++ {
				t := float64(k) / 5
				px := int(bx + math.Cos(ang)*L*t*0.9 + s)
				py := int(floor - math.Sin(ang+0.5)*L*t*0.5 - float64(h)*0.02)
				if px >= 0 && px < w && py >= zoneTop && py < h {
					base := img.RGBAAt(px, py)
					img.Set(px, py, color.RGBA{
						R: lerpU8(base.R, 25, 0.5),
						G: lerpU8(base.G, 90, 0.55),
						B: lerpU8(base.B, 65, 0.5),
						A: 255,
					})
				}
			}
		}
	}

	// Light bottom fog (cheap row blend)
	for y := h - max(6, h/48); y < h; y++ {
		if y < 0 {
			continue
		}
		t := float64(h-y) / float64(max(6, h/48))
		if t > 1 {
			t = 1
		}
		for x := 0; x < w; x++ {
			base := img.RGBAAt(x, y)
			img.Set(x, y, color.RGBA{
				R: lerpU8(base.R, 6, t*0.28),
				G: lerpU8(base.G, 16, t*0.32),
				B: lerpU8(base.B, 22, t*0.36),
				A: 255,
			})
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func paintBlackHole(img *image.RGBA, cx, cy, r int, spin float64) {
	if r < 6 {
		r = 6
	}
	b := img.Bounds()
	rDisk := float64(r) * 1.55
	rHalo := float64(r) * 2.35

	for y := cy - int(rHalo) - 2; y <= cy+int(rHalo)+2; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := cx - int(rHalo) - 2; x <= cx+int(rHalo)+2; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			dx := float64(x - cx)
			dy := float64(y - cy)
			d := math.Hypot(dx, dy)
			ang := math.Atan2(dy, dx)

			base := img.RGBAAt(x, y)
			ringBoost := math.Exp(-math.Abs(d-float64(r)*1.15)/(float64(r)*0.09)) * 0.45

			ex := d * (0.92 + 0.08*math.Cos(ang*2+spin*1.7))
			diskHot := math.Exp(-math.Abs(ex-float64(r)*1.05) / (float64(r) * 0.14))
			warp := 0.55 + 0.45*math.Sin(ang*3+spin*2.2+math.Sin(spin+ang))
			hr := uint8(200 + 55*warp*diskHot)
			hg := uint8(60 + 120*diskHot*warp)
			hb := uint8(180 + 75*diskHot*(1-warp*0.5))

			tDisk := diskHot * 0.55 * (0.4 + 0.6*warp)
			if tDisk > 0.92 {
				tDisk = 0.92
			}

			if d < float64(r)*0.42 {
				tCore := 1 - d/(float64(r)*0.42)
				dark := color.RGBA{R: 2, G: 4, B: 14}
				img.Set(x, y, color.RGBA{
					R: lerpU8(base.R, dark.R, 0.25+tCore*0.65),
					G: lerpU8(base.G, dark.G, 0.25+tCore*0.65),
					B: lerpU8(base.B, dark.B, 0.25+tCore*0.65),
					A: 255,
				})
				continue
			}

			if d < float64(r)*0.52 {
				rim := (d - float64(r)*0.42) / (float64(r) * 0.1)
				tRim := (1 - rim) * 0.85
				img.Set(x, y, color.RGBA{
					R: lerpU8(base.R, 240, tRim*0.35),
					G: lerpU8(base.G, 220, tRim*0.45),
					B: lerpU8(base.B, 255, tRim*0.5),
					A: 255,
				})
				continue
			}

			if d < rDisk && ex < float64(r)*1.35 {
				img.Set(x, y, color.RGBA{
					R: lerpU8(base.R, hr, tDisk),
					G: lerpU8(base.G, hg, tDisk),
					B: lerpU8(base.B, hb, tDisk),
					A: 255,
				})
				continue
			}

			if d < rHalo && d > rDisk {
				th := math.Exp(-(d-rDisk)/(float64(r)*0.5)) * ringBoost * 0.35
				if th > 0.04 {
					img.Set(x, y, color.RGBA{
						R: lerpU8(base.R, 100, th),
						G: lerpU8(base.G, 140, th),
						B: lerpU8(base.B, 220, th),
						A: 255,
					})
				}
				continue
			}
		}
	}
}

func paintMeteor(img *image.RGBA, w, h int, m meteor) {
	dx, dy := m.vx, m.vy
	norm := math.Hypot(dx, dy)
	if norm < 1e-6 {
		return
	}
	dx /= norm
	dy /= norm
	headX, headY := m.x, m.y
	tailLen := m.len * m.life
	const steps = 36
	for s := 0; s < steps; s++ {
		t := float64(s) / float64(steps-1)
		bright := m.life * (1 - t) * (1 - t)
		if bright < 0.02 {
			continue
		}
		px := int(headX - dx*tailLen*t)
		py := int(headY - dy*tailLen*t)
		if px < 0 || py < 0 || px >= w || py >= h {
			continue
		}
		base := img.RGBAAt(px, py)
		cr := uint8(180 + 75*bright)
		cg := uint8(220 + 35*bright)
		cb := uint8(255)
		img.Set(px, py, color.RGBA{
			R: lerpU8(base.R, cr, bright*0.95),
			G: lerpU8(base.G, cg, bright*0.9),
			B: lerpU8(base.B, cb, bright*0.85),
			A: 255,
		})
	}
	for ox := -1; ox <= 1; ox++ {
		for oy := -1; oy <= 1; oy++ {
			px := int(headX) + ox
			py := int(headY) + oy
			if px < 0 || py < 0 || px >= w || py >= h {
				continue
			}
			base := img.RGBAAt(px, py)
			f := m.life * 0.9
			img.Set(px, py, color.RGBA{
				R: lerpU8(base.R, 255, f),
				G: lerpU8(base.G, 250, f),
				B: lerpU8(base.B, 255, f),
				A: 255,
			})
		}
	}
}

func drawCometScaled(img *image.RGBA, w, h int, c cometState, phase float64) {
	md := minDim(w, h)
	cx := float64(w)*0.5 + math.Cos(c.angle)*float64(md)*c.dist
	cy := float64(h)*0.38 + math.Sin(c.angle*0.9)*float64(md)*0.18
	tx := math.Cos(c.angle+math.Pi) * float64(md) * c.tailLen
	ty := math.Sin(c.angle+math.Pi)*float64(md)*c.tailLen*0.65 + math.Sin(phase*0.8)*4

	for s := 0; s < 48; s++ {
		t := float64(s) / 47
		bright := c.brightness * (1 - t) * (1 - t) * (1 - t)
		if bright < 0.03 {
			continue
		}
		px := int(cx + tx*t*0.85)
		py := int(cy + ty*t*0.85)
		if px < 0 || py < 0 || px >= w || py >= h {
			continue
		}
		base := img.RGBAAt(px, py)
		img.Set(px, py, color.RGBA{
			R: lerpU8(base.R, 200, bright*0.7),
			G: lerpU8(base.G, 230, bright*0.75),
			B: lerpU8(base.B, 255, bright*0.8),
			A: 255,
		})
	}
	for ox := -2; ox <= 2; ox++ {
		for oy := -2; oy <= 2; oy++ {
			if ox*ox+oy*oy > 8 {
				continue
			}
			px := int(cx) + ox
			py := int(cy) + oy
			if px < 0 || py < 0 || px >= w || py >= h {
				continue
			}
			base := img.RGBAAt(px, py)
			f := c.brightness * 0.5
			img.Set(px, py, color.RGBA{
				R: lerpU8(base.R, 255, f),
				G: lerpU8(base.G, 245, f),
				B: lerpU8(base.B, 220, f),
				A: 255,
			})
		}
	}
}

func paintOrb(img *image.RGBA, cx, cy, r int, tr, tg, tb uint8, peak float64) {
	b := img.Bounds()
	r2 := float64(r * r)
	for y := cy - r; y <= cy+r; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := cx - r; x <= cx+r; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			dx := float64(x - cx)
			dy := float64(y - cy)
			d2 := dx*dx + dy*dy
			if d2 > r2 {
				continue
			}
			t := (1 - d2/r2) * peak
			if t <= 0 {
				continue
			}
			if t > 1 {
				t = 1
			}
			base := img.RGBAAt(x, y)
			img.Set(x, y, color.RGBA{
				R: lerpU8(base.R, tr, t),
				G: lerpU8(base.G, tg, t),
				B: lerpU8(base.B, tb, t),
				A: 255,
			})
		}
	}
}

func lerpU8(a, b uint8, t float64) uint8 {
	return uint8(float64(a)*(1-t) + float64(b)*t + 0.5)
}

func minDim(w, h int) int {
	if w < h {
		return w
	}
	return h
}
