# Dashboards

!!! status-wip "In development"
    The WYSIWYG editor round-trip (edit in UI → writes back to Pkl) is not yet shipped. Dashboard declaration in Pkl and rendering in the web UI are functional; the write-back path is in active development.

Dashboards are declared in `dashboards.pkl` as a tree of pages, grids, and widget instances. The web UI renders them directly from the current `ConfigSnapshot`.

## Dashboard structure

A dashboard is a slug-addressed collection of pages. Each page has a title and a grid of widget instances. Widgets are positioned on the grid with column, row, width, and height values.

```pkl
// dashboards.pkl
import "switchyard:dashboards" as dashboards
import "switchyard:widgets"    as widgets

dashboards: Listing<dashboards.Dashboard> = new {
  new dashboards.Dashboard {
    slug  = "main"
    pages = new {

      new dashboards.Page {
        title = "Overview"
        grid  = new dashboards.Grid {
          widgets = new {

            // Temperature sensor — spans 2 columns, 1 row, top-left
            new dashboards.WidgetInstance {
              widgetClass = widgets.gauge
              col = 0; row = 0; w = 2; h = 1
              props = new {
                ["entityId"]  = "sensor.living_room_temp"
                ["unit"]      = "°C"
                ["min"]       = 15.0
                ["max"]       = 30.0
              }
            }

            // Light toggle — 1×1, next column
            new dashboards.WidgetInstance {
              widgetClass = widgets.entityToggle
              col = 2; row = 0; w = 1; h = 1
              props = new {
                ["entityId"]     = "light.kitchen_ceiling"
                ["label"]        = "Kitchen Ceiling"
                ["showDimmer"]   = true
              }
            }

            // Energy history — full-width, second row
            new dashboards.WidgetInstance {
              widgetClass = widgets.lineChart
              col = 0; row = 1; w = 4; h = 2
              props = new {
                ["entityId"] = "sensor.whole_home_power"
                ["label"]    = "Power draw (W)"
                ["window"]   = "24h"
              }
            }

          }
        }
      }

      new dashboards.Page {
        title = "Upstairs"
        grid  = new dashboards.Grid {
          widgets = new {
            new dashboards.WidgetInstance {
              widgetClass = widgets.entityToggle
              col = 0; row = 0; w = 1; h = 1
              props = new {
                ["entityId"] = "light.upstairs_landing"
                ["label"]    = "Landing"
              }
            }
          }
        }
      }

    }
  }
}
```

## Pkl data types

The `switchyard:dashboards` module:

```pkl
module switchyard.dashboards

class WidgetInstance {
  widgetClass: String              // widget type constant from switchyard:widgets
  props: Mapping<String, Any>      // widget-specific props (typed by the widget)
  col: Int; row: Int               // grid position (0-indexed)
  w: Int;   h: Int                 // grid span
}

class Grid   { widgets: Listing<WidgetInstance> }
class Page   { title: String; grid: Grid }
class Dashboard { slug: String; pages: Listing<Page> }
```

The `switchyard:widgets` module provides string constants for the standard widget class names:

```pkl
module switchyard.widgets

const gauge:        String = "Gauge"
const lineChart:    String = "LineChart"
const entityToggle: String = "EntityToggle"
const markdown:     String = "Markdown"
const scriptButton: String = "ScriptButton"
```

Using these constants reduces typo risk — the string values are well-known names from the standard widget pack.

## Standard widget pack

switchyard ships a set of standard widgets:

| Widget | `widgetClass` | Key props |
|---|---|---|
| `Gauge` | `widgets.gauge` | `entityId`, `unit`, `min`, `max` |
| `LineChart` | `widgets.lineChart` | `entityId`, `label`, `window` |
| `EntityToggle` | `widgets.entityToggle` | `entityId`, `label`, `showDimmer` |
| `Markdown` | `widgets.markdown` | `content` (static Markdown string) |
| `ScriptButton` | `widgets.scriptButton` | `scriptId`, `label`, `confirmLabel` |

## WYSIWYG round-trip (planned)

The goal is for the web UI's drag-and-drop dashboard editor to write changes back to `dashboards.pkl` on disk. When you move a widget or change its props in the UI, the daemon serializes the updated dashboard Pkl and writes it to your config directory, so the change is immediately git-trackable.

This requires the daemon to hold a write lock on the config directory and to understand the Pkl formatting conventions for dashboard files. It is in active development. Until it ships, dashboards are edited by hand in Pkl and re-applied with `switchyard config apply`.
