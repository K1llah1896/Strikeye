package settings

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"strikeye/internal/mapassets"
	"strikeye/internal/toolbundle"
	"strikeye/internal/ui/theme"
)

// renderMapDataCard shows the state of the bundled navmesh/callout caches. The
// datasets ship inside the package, so nothing here is a prerequisite; the
// buttons only re-install from the bundle or refresh from upstream.
func (v *View) renderMapDataCard(c *ui.Context) {
	if v.mapDataCheckedAt.IsZero() {
		v.refreshMapDataStatus()
	}
	bundled := mapassets.Available()
	bundlePath, hasBundle := mapassets.FindGeometryBundle(v.dataRootOf(), mapassets.ExeDir())
	theme.BentoCard(c, ui.Color{}, 12, func() {
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			theme.RenderIcon(c, theme.IconRadar, 15, theme.AccentCyan)
			theme.SectionTitle(c, "地图数据（导航网格与 callout）")
			ui.Spacer(c)
			if v.mapDataBusy {
				theme.PillBadge(c, "处理中...", theme.AccentAmber)
			}
			if theme.IconActionButton(c, theme.IconRefresh, "刷新状态").Clicked() {
				v.refreshMapDataStatus()
			}
		})
		theme.MutedText(c, fmt.Sprintf("已缓存导航网格 %d 张 · 碰撞几何 %d 张 · callout %s · 检查于 %s",
			v.mapDataNavCount, v.mapDataGeometry, humanBytes(v.mapDataCallouts), v.mapDataCheckedAt.Format("15:04:05")))
		switch {
		case bundled && hasBundle:
			theme.MutedText(c, "内置离线数据：导航网格与 callout 随程序提供，碰撞几何从 map_geometry.zip 按需解压（"+shortPath(bundlePath)+"），无需联网。")
		case bundled:
			theme.MutedText(c, "内置离线数据：导航网格与 callout 随程序提供；未找到 map_geometry.zip，TTD 与击杀视线会在缺失时自动降级。")
		default:
			theme.MutedText(c, "本构建未内置地图数据（源码构建）：可先跑 `strikeye-cli assets-pack` 后重构建，或直接用下方按钮联网补齐。")
		}
		if v.mapDataNavCount == 0 || v.mapDataGeometry == 0 || v.mapDataCallouts == 0 {
			theme.MutedText(c, "区域控制/地图控制需要导航网格，TTD 与击杀视线需要碰撞几何，地名需要 callout；缺失时对应区块会提示但不报错。")
		}
		theme.RowCenter(c).FillWidth().Gap(8).Children(func() {
			if bundled && theme.ActionButton(c, "从内置包安装").Clicked() && !v.mapDataBusy {
				v.startMapDataTask("内置数据", func(ctx context.Context) (string, error) {
					return installBundledMapAssets(v.dataRootOf())
				})
			}
			// Network refreshes are a source-build fallback: a packaged release
			// already contains every dataset, so they are hidden from users.
			if !bundled && theme.ActionButton(c, "下载 callout").Clicked() && !v.mapDataBusy {
				v.startMapDataTask("callout", func(ctx context.Context) (string, error) {
					path, err := fetchCallouts(ctx, v.dataRootOf())
					if err != nil {
						return "", err
					}
					return "callout 已更新: " + path, nil
				})
			}
			if !bundled && theme.ActionButton(c, "下载碰撞几何（全部官方图）").Clicked() && !v.mapDataBusy {
				v.startMapDataTask("geometry", func(ctx context.Context) (string, error) {
					files, err := fetchGeometry(ctx, v.dataRootOf(), nil)
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("碰撞几何已就绪 %d 张", len(files)), nil
				})
			}
			if !bundled && theme.ActionButton(c, "下载导航网格（全部官方图）").Clicked() && !v.mapDataBusy {
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

// installBundledMapAssets unpacks the embedded nav meshes, the callout table and
// (when the sidecar archive is present) every collision geometry file.
func installBundledMapAssets(dataRoot string) (string, error) {
	rep, err := mapassets.Install(dataRoot, mapassets.ExeDir())
	if err != nil {
		return "", err
	}
	geometry := 0
	if rep.GeometryBundle != "" {
		files, err := mapassets.ExtractAllGeometry(rep.GeometryBundle, filepath.Join(dataRoot, "cache", "map_geometry"))
		if err != nil {
			return "", err
		}
		geometry = len(files)
	}
	summary := fmt.Sprintf("内置数据已安装：导航网格 %d 张 · callout %d · 碰撞几何 %d 张",
		rep.NavsWritten+rep.NavsPresent, boolToInt(rep.CalloutsWritten), geometry)
	if tools, err := toolbundle.Provision(dataRoot, mapassets.ExeDir()); err != nil {
		summary += " · 工具安装失败：" + err.Error()
	} else if len(tools.Installed) > 0 {
		summary += fmt.Sprintf(" · 工具 %d 个（%s）", len(tools.Installed), strings.Join(tools.Installed, "/"))
	}
	return summary, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// shortPath trims a long absolute path to its last two segments for the UI.
func shortPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path))
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
