package settings

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/ui/theme"
)

// renderMapDataCard shows the state of the downloaded navmesh/callout caches and
// offers the same download the analysis tabs tell users to run in the CLI.
func (v *View) renderMapDataCard(c *ui.Context) {
	if v.mapDataCheckedAt.IsZero() {
		v.refreshMapDataStatus()
	}
	theme.BentoCard(c, ui.Color{}, 12, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.RenderIcon(c, theme.IconRadar, 15, theme.AccentCyan)
			theme.SectionTitle(c, "地图数据（导航网格与 callout）")
			ui.Spacer(c)
			if v.mapDataBusy {
				theme.PillBadge(c, "下载中...", theme.AccentAmber)
			}
			if theme.IconActionButton(c, theme.IconRefresh, "刷新状态").Clicked() {
				v.refreshMapDataStatus()
			}
		})
		theme.MutedText(c, fmt.Sprintf("已缓存导航网格 %d 张 · 碰撞几何 %d 张 · callout %s · 检查于 %s",
			v.mapDataNavCount, v.mapDataGeometry, humanBytes(v.mapDataCallouts), v.mapDataCheckedAt.Format("15:04:05")))
		if v.mapDataNavCount == 0 || v.mapDataGeometry == 0 || v.mapDataCallouts == 0 {
			theme.MutedText(c, "区域控制/地图控制需要导航网格，TTD 与击杀视线需要碰撞几何，地名需要 callout；缺失时对应区块会提示但不报错。")
		}
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			if theme.ActionButton(c, "下载 callout").Clicked() && !v.mapDataBusy {
				v.startMapDataTask("callout", func(ctx context.Context) (string, error) {
					path, err := fetchCallouts(ctx, v.dataRootOf())
					if err != nil {
						return "", err
					}
					return "callout 已更新: " + path, nil
				})
			}
			if theme.ActionButton(c, "下载碰撞几何（全部官方图）").Clicked() && !v.mapDataBusy {
				v.startMapDataTask("geometry", func(ctx context.Context) (string, error) {
					files, err := fetchGeometry(ctx, v.dataRootOf(), nil)
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("碰撞几何已就绪 %d 张", len(files)), nil
				})
			}
			if theme.ActionButton(c, "下载导航网格（全部官方图）").Clicked() && !v.mapDataBusy {
				v.startMapDataTask("navs", func(ctx context.Context) (string, error) {
					files, err := fetchNavs(ctx, v.dataRootOf(), nil)
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("导航网格已就绪 %d 张", len(files)), nil
				})
			}
		})
	})
}

// dataRootOf is the DataRoot this settings view was created with.
func (v *View) dataRootOf() string { return v.dataRoot }

// refreshMapDataStatus samples the cache contents for the status line.
func (v *View) refreshMapDataStatus() {
	navs, callouts := mapDataStatus(v.dataRootOf())
	v.mapDataNavCount = len(navs)
	v.mapDataGeometry = geometryCount(v.dataRootOf())
	v.mapDataCallouts = callouts
	v.mapDataCheckedAt = time.Now()
}

// startMapDataTask runs one download in the background and reports through the
// shared status bar, so the settings render loop never blocks on the network.
func (v *View) startMapDataTask(label string, run func(context.Context) (string, error)) {
	v.mapDataBusy = true
	v.setStatus("正在下载 " + label + "...")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mapDataBusyTimeout)
		defer cancel()
		msg, err := run(ctx)
		v.updateMu.Lock()
		v.mapDataBusy = false
		v.updateMu.Unlock()
		if err != nil {
			v.setStatus("下载失败: " + err.Error())
		} else {
			v.setStatus(msg)
		}
		navs, callouts := mapDataStatus(v.dataRoot)
		geometries := geometryCount(v.dataRoot)
		v.updateMu.Lock()
		v.mapDataNavCount = len(navs)
		v.mapDataGeometry = geometries
		v.mapDataCallouts = callouts
		v.mapDataCheckedAt = time.Now()
		v.updateMu.Unlock()
		v.invalidate()
	}()
}

// humanBytes renders a cache size for the status line.
func humanBytes(n int64) string {
	switch {
	case n <= 0:
		return "缺失"
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
