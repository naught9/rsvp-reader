package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// chromeIdleHide is how long playback runs before the chrome melts away.
const chromeIdleHide = 2500 * time.Millisecond

// activityDetector sits behind the reader content and reports mouse
// movement (to reveal the chrome) and taps (zen click to pause/resume).
// Controls above it receive their own events first.
type activityDetector struct {
	widget.BaseWidget
	onMove func()
	onTap  func()
}

func newActivityDetector(onMove, onTap func()) *activityDetector {
	d := &activityDetector{onMove: onMove, onTap: onTap}
	d.ExtendBaseWidget(d)
	return d
}

func (d *activityDetector) CreateRenderer() fyne.WidgetRenderer {
	return &detectorRenderer{d: d}
}

// MouseIn reports activity.
func (d *activityDetector) MouseIn(*desktop.MouseEvent) { d.moved() }

// MouseMoved reports activity.
func (d *activityDetector) MouseMoved(*desktop.MouseEvent) { d.moved() }

// MouseOut is ignored.
func (d *activityDetector) MouseOut() {}

func (d *activityDetector) moved() {
	if d.onMove != nil {
		d.onMove()
	}
}

// Tapped toggles playback in zen fashion.
func (d *activityDetector) Tapped(*fyne.PointEvent) {
	if d.onTap != nil {
		d.onTap()
	}
}

type detectorRenderer struct {
	d *activityDetector
}

func (r *detectorRenderer) MinSize() fyne.Size { return fyne.NewSize(1, 1) }
func (r *detectorRenderer) Layout(fyne.Size)   {}
func (r *detectorRenderer) Refresh()           {}
func (r *detectorRenderer) Objects() []fyne.CanvasObject {
	return nil
}
func (r *detectorRenderer) Destroy() {}

var _ desktop.Hoverable = (*activityDetector)(nil)
var _ fyne.Tappable = (*activityDetector)(nil)

// mirrorSpacer reserves exactly the space its target occupies, so the
// chrome can overlay reserved gutters: hiding chrome never moves content.
type mirrorSpacer struct {
	widget.BaseWidget
	target fyne.CanvasObject
}

func newMirrorSpacer(target fyne.CanvasObject) *mirrorSpacer {
	m := &mirrorSpacer{target: target}
	m.ExtendBaseWidget(m)
	return m
}

func (m *mirrorSpacer) CreateRenderer() fyne.WidgetRenderer {
	return &mirrorSpacerRenderer{m: m}
}

type mirrorSpacerRenderer struct {
	m *mirrorSpacer
}

func (r *mirrorSpacerRenderer) MinSize() fyne.Size {
	// Height follows the chrome; width is irrelevant (border stretches it).
	// Visibility does not affect MinSize, so hide/show cycles are stable.
	return fyne.NewSize(1, r.m.target.MinSize().Height)
}
func (r *mirrorSpacerRenderer) Layout(fyne.Size) {}
func (r *mirrorSpacerRenderer) Refresh()         {}
func (r *mirrorSpacerRenderer) Objects() []fyne.CanvasObject {
	return nil
}
func (r *mirrorSpacerRenderer) Destroy() {}

// poke records mouse activity: the chrome appears. While playing the
// idle timer that melts it away is re-armed; while paused the chrome
// holds with no timer, so it can never auto-hide when paused.
func (a *App) poke() {
	if a.book == nil {
		return
	}
	a.showChrome()
	a.cancelHideTimer()
	if a.player != nil && a.player.Playing() {
		a.hideTimer = time.AfterFunc(chromeIdleHide, func() {
			fyne.Do(a.hideChrome)
		})
	}
}

// hideNow melts the chrome immediately, whatever the state.

func (a *App) showChrome() {
	if !a.chromeHidden {
		return
	}
	a.chromeHidden = false
	if a.topWrap != nil {
		a.topWrap.Show()
	}
	if a.bottomWrap != nil {
		a.bottomWrap.Show()
	}
}

func (a *App) hideChrome() {
	if a.chromeHidden || a.book == nil {
		return
	}
	if a.player != nil && !a.player.Playing() {
		return // stale timer: pausing outranks a pending hide
	}
	a.chromeHidden = true
	if a.topWrap != nil {
		a.topWrap.Hide()
	}
	if a.bottomWrap != nil {
		a.bottomWrap.Hide()
	}
}

func (a *App) cancelHideTimer() {
	if a.hideTimer != nil {
		a.hideTimer.Stop()
		a.hideTimer = nil
	}
}

// revealChrome shows the chrome with no idle timer: the paused holding
// state. Paused chrome never auto-hides.
func (a *App) revealChrome() {
	a.showChrome()
	a.cancelHideTimer()
}

// hideNow melts the chrome immediately, whatever the state.
func (a *App) hideNow() {
	a.cancelHideTimer()
	a.hideChrome()
}
