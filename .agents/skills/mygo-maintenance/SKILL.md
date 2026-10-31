---
name: mygo-maintenance
description: Maintain Go applications using github.com/egoist/mygo, including UI changes, crash fixes, and dependency upgrades. Apply MyGo-specific lifetime, identity, threading, and testing practices.
---

# MyGo Application Maintenance & Design Specification

This skill provides guidelines, architecture rules, and UI design standards for building and maintaining native Go desktop applications powered by [MyGo](https://github.com/egoist/mygo).

## Official References & Documentation

When designing new interfaces, debugging framework behavior, or verifying idioms, consult these official references:

- **Official Documentation**: [https://mygo.egoist.dev/docs](https://mygo.egoist.dev/docs)
- **UI System & Widget Guide**: [https://mygo.egoist.dev/docs/ui](https://mygo.egoist.dev/docs/ui)
- **Official Example Projects**: [https://github.com/egoist/mygo/tree/main/examples](https://github.com/egoist/mygo/tree/main/examples)
  - Reference patterns for sidebars, split views, toolbars, file managers, editors, and dialogs.
- **Source Repository**: [https://github.com/egoist/mygo](https://github.com/egoist/mygo)
- **Local Pinned Source**: Locate with `go list -m -f '{{.Dir}}' github.com/egoist/mygo`; read `ui/doc.go`, `ui/widgets.go`, `ui/theme.go`, and relevant implementation comments. Confirm API availability against the pinned version before writing code.

---

## UI Architecture & Design Standards

### 1. Visual Aesthetics & Theme Integration
- **Theme-First Styling**: Always query `t := c.Theme()`. Use semantic tokens: `t.Background`, `t.Surface`, `t.SurfaceHover`, `t.Border`, `t.Text`, `t.TextMuted`, `t.Accent`, `t.AccentHover`, `t.AccentPressed`, `t.AccentText`, and `t.Radius`.
- **Adaptive Light/Dark Modes**: Never hardcode static light-only or dark-only hex colors for surfaces and text. If custom palette extensions are required, implement an adaptive helper (e.g. `paletteFor(t *ui.Theme)`) that returns distinct, high-contrast, harmonious colors for both `t.Dark == false` and `t.Dark == true`.
- **Vector Icons over Bitmaps**:
  - Pre-parse SVG assets once at package initialization using `ui.MustParseSVG([]byte(...))` or `ui.ParseSVG`.
  - Render via `ui.Icon(c, svg)` for single-color icons matching current text color, or `ui.Image(c, svg)` for multi-color illustrations.
  - Never parse SVGs inside `view(c *ui.Context)` or rendering passes.
- **Visual Rhythm & Spacing**:
  - Base spacing on `t.Space(...)` or multiples of 4/8px grid.
  - Group related actions with `ui.Divider` or segmented containers.
  - Use subtle borders (`t.Border` with opacity or 1px width) and card container radii (`Radius(6)` to `Radius(10)`) to create clear visual hierarchy without visual clutter.
  - Employ status pills/badges (e.g. `ui.Row(c).Padding(2, 8).Radius(999)...`) to present metadata, diff counts, and active states cleanly.

### 2. Layout Structure & Responsive Design
- **Shell Layout**: Standard desktop apps should utilize a high-craft sidebar (`ui.Sidebar` / `ui.Column` with custom item highlights), a top action/navigation bar, a contextual control header, and a responsive workspace area.
- **Responsive Viewport Support**: Ensure layouts fit both minimum supported sizes (e.g. 800–900px width) and wide displays (1380px+) without button clipping or uncontrolled overflow. Critical action buttons must remain accessible across all supported viewport bounds.
- **Layout Containers**: Use `ui.Row`, `ui.Column`, and `ui.Box` appropriately:
  - Use `.Grow(1).Basis(0)` for balanced multi-pane side-by-side columns.
  - Use `.MinWidth(0)` or `.Shrink(1)` on text containers inside flex rows to avoid blowing out container widths on long paths.
  - Use `ui.SplitVerticalPercent` or `ui.SplitHorizontalPercent` when user-adjustable resizable panes are required.

---

## UI Lifetime & Identity

- **Context Scope**: Views receive a stable `*ui.Context` for their window. `Children(func())` shares it and changes its parent scope during the callback. Keep `*ui.Context` out of persistent app state and background goroutines; capture `c.Services()` for persistent clipboard/URL callbacks and background updates.
- **Element Ephemerality**: Constructors return checked `ui.Element` values. Elements and custom parts expire before the next build pass, including another pass in the same frame. Store application data, widget state, and `ui.Handle`, never an `Element` or parts.
- **Stable Identity**:
  - Assign stable identity before widget construction using `c.Key(key)` for dynamic or conditional elements.
  - When rendering lists or tables with `ui.List`, configure `.ItemKey(func(i int) any { ... })` to provide stable item keys across filtering and sorting passes.
- **Handles & Focus**:
  - Bind persistent identity with `.Bind(&handle)`.
  - Use `handle.Focused(c)` to check focus state, `handle.Focus(c)` to request focus, and `handle.Bounds(c)` to read committed geometry in input callbacks.
  - Do not capture an element in a later input callback to query its bounds.

---

## Input & Actions

- **Bound Values & Change Notifications**:
  - Bound values update after construction and configuration. `Changed` / `Submitted` notices appear in the following build pass. Bind directly to persistent model fields when possible.
  - Write derived checkbox or segmented values back in `OnClick`, after that build's bound input updates them.
  - Prefer deferred actions when mutating collections that are actively being built or rendered.
- **Keyboard Shortcuts**:
  - Declare window-level shortcuts with `c.OnShortcut(ui.Cmd, key, fn)`.
  - Declare scoped widget commands with `handle.OnShortcut(c, mods, key, fn)`. Only enabled controls present in the current build receive the command.

---

## State, Concurrency & Rendering Performance

- **Zero Heavy Work in Views**:
  - Views may rebuild multiple times per second during interactions and animations. Never perform disk I/O, network requests, or heavy computations inside `view(c *ui.Context)` or child layout callbacks.
- **Background Jobs & Thread Hopping**:
  - Run asynchronous tasks in goroutines and pass results back to the UI thread via `Window.Update(func())`.
  - Support job cancellation (`context.WithCancel`) when inputs change rapidly or when switching tabs/modes, preventing stale results from overwriting newer user state.
- **Purity in Custom Painters**:
  - Keep canvas and drawing callbacks limited strictly to painting; do not mutate application model state inside painters.

---

## Verification & Testing Standards

- **Headless UI Testing with `ui.NewTester`**:
  - Verify layout boundaries, button clicks, typing, and keyboard shortcuts using `ui.NewTester(view, width, height)`.
  - For tests asserting text, labels, or bounds, ensure elements define explicit `.Label("...")` where appropriate.
- **Cross-Theme Verification**:
  - Test both light and dark modes (`tt.SetDark(false)` and `tt.SetDark(true)`).
  - When snapshot testing is enabled, output and inspect rendered images to ensure contrast, alignment, and aesthetic quality.
- **Static & Concurrency Checks**:
  - Always run `go tool mygo vet .` to catch stored element lifetimes or improper context captures.
  - Run `go vet ./...` and `CGO_ENABLED=0 go test ./...`.
  - Run `go test -race ./...` on any package with asynchronous background jobs or shared state.
