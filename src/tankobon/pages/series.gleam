import gleam/bool
import gleam/int
import gleam/json
import gleam/list
import gleam/option
import gleam/result
import gleam/string
import lustre
import lustre/attribute
import lustre/component
import lustre/effect
import lustre/element
import lustre/element/html
import lustre/event
import modem
import plinth/javascript/global
import plinth/javascript/storage
import rsvp
import tankobon/api/auth
import tankobon/api/series as series_api
import tankobon/ui/badge
import tankobon/ui/button
import tankobon/ui/display
import tankobon/ui/modal
import tankobon/ui/toast

const release_poll_interval_ms = 2500

const toast_dismiss_ms = 5000

pub type Model {
  Model(
    account: auth.Account,
    series_id: Int,
    series: option.Option(series_api.Series),
    loading: Bool,
    show_delete_modal: Bool,
    delete_files: Bool,
    show_search_modal: Bool,
    release_search: option.Option(series_api.ReleaseSearch),
    toast: option.Option(toast.Toast),
  )
}

fn int_range(from: Int, to: Int) -> List(Int) {
  case from > to {
    True -> []
    False -> [from, ..int_range(from + 1, to)]
  }
}

pub type Msg {
  SetSeriesId(Int)
  SeriesResponse(Result(series_api.Series, rsvp.Error(String)))
  ShowDeleteModal
  ToggleDeleteFiles
  CancelDelete
  DeleteSeries
  SearchSeries
  RefreshSeries
  SearchSeriesResponse(Result(Nil, rsvp.Error(String)))
  DeleteResponse(Result(Nil, rsvp.Error(String)))
  ToggleSeriesMonitor(Int, Bool)
  ToggleVolumeMonitor(Int, String, Bool)
  ToggleChapterMonitor(Int, Bool)
  PatchResponse(Result(series_api.Series, rsvp.Error(String)))
  OpenInteractiveSearch
  CloseSearchModal
  FindReleasesResponse(Result(series_api.ReleaseSearch, rsvp.Error(String)))
  PollReleaseSearch
  ReleaseSearchResponse(Result(series_api.ReleaseSearch, rsvp.Error(String)))
  GrabRelease(String, String)
  GrabResponse(Result(Nil, rsvp.Error(String)), String)
  DismissToast
}

pub fn register() {
  let app =
    lustre.component(init, update, view, [
      component.on_attribute_change("series-id", fn(val) {
        int.parse(val)
        |> result.map(SetSeriesId)
        |> result.map_error(fn(_) { Nil })
      }),
    ])
  lustre.register(app, "series-page")
}

pub fn element(id: Int) {
  element.element(
    "series-page",
    [attribute.attribute("series-id", int.to_string(id))],
    [],
  )
}

fn init(_) {
  let assert Ok(stg) = storage.local()
  let assert Ok(account_json) = stg |> storage.get_item("tankobon_account")
  let assert Ok(account) = json.parse(account_json, auth.account_decoder())
  #(
    Model(
      account:,
      series_id: 0,
      series: option.None,
      loading: False,
      show_delete_modal: False,
      delete_files: False,
      show_search_modal: False,
      release_search: option.None,
      toast: option.None,
    ),
    effect.none(),
  )
}

fn schedule_release_poll() -> effect.Effect(Msg) {
  effect.from(fn(dispatch) {
    global.set_timeout(release_poll_interval_ms, fn() {
      dispatch(PollReleaseSearch)
    })
    Nil
  })
}

fn schedule_toast_dismiss() -> effect.Effect(Msg) {
  effect.from(fn(dispatch) {
    global.set_timeout(toast_dismiss_ms, fn() { dispatch(DismissToast) })
    Nil
  })
}

fn update(m: Model, msg: Msg) {
  case msg {
    SetSeriesId(id) -> #(
      Model(..m, series_id: id, loading: True),
      series_api.get(id, m.account.access_token, SeriesResponse),
    )
    SeriesResponse(Ok(s)) -> #(
      Model(..m, series: option.Some(s), loading: False),
      effect.none(),
    )
    SeriesResponse(Error(_)) -> #(Model(..m, loading: False), effect.none())
    ShowDeleteModal -> #(
      Model(..m, show_delete_modal: True, delete_files: False),
      effect.none(),
    )
    ToggleDeleteFiles -> #(
      Model(..m, delete_files: !m.delete_files),
      effect.none(),
    )
    CancelDelete -> #(Model(..m, show_delete_modal: False), effect.none())
    RefreshSeries -> #(
      m,
      series_api.refresh_metadata(
        m.series_id,
        m.account.access_token,
        SeriesResponse,
      ),
    )
    DeleteSeries -> #(
      Model(..m, show_delete_modal: False),
      series_api.delete(
        m.series_id,
        m.account.access_token,
        m.delete_files,
        DeleteResponse,
      ),
    )
    DeleteResponse(Ok(_)) -> #(m, modem.push("/", option.None, option.None))
    DeleteResponse(Error(_)) -> #(m, effect.none())
    ToggleSeriesMonitor(id, currently_monitored) -> {
      let body =
        json.object([
          #("monitored", json.bool(currently_monitored |> bool.negate)),
        ])
      #(m, series_api.patch(id, body, m.account.access_token, PatchResponse))
    }
    ToggleVolumeMonitor(id, vol, is_currently_unmonitored) ->
      case m.series {
        option.None -> #(m, effect.none())
        option.Some(s) -> {
          let new_unmonitored = case is_currently_unmonitored {
            True ->
              s.unmonitored_volumes
              |> list.filter(fn(v) { v != vol })
            False -> [vol, ..s.unmonitored_volumes]
          }
          let body =
            json.object([
              #("unmonitoredVolumes", json.array(new_unmonitored, json.string)),
            ])
          #(
            m,
            series_api.patch(id, body, m.account.access_token, PatchResponse),
          )
        }
      }
    ToggleChapterMonitor(id, currently_monitored) -> {
      let body =
        json.object([
          #("monitorChapters", json.bool(currently_monitored |> bool.negate)),
        ])
      #(m, series_api.patch(id, body, m.account.access_token, PatchResponse))
    }
    SearchSeries -> #(
      m,
      series_api.search_on_series(
        m.series_id,
        m.account.access_token,
        SearchSeriesResponse,
      ),
    )
    SearchSeriesResponse(_) -> #(m, effect.none())
    PatchResponse(Ok(updated)) -> #(
      Model(..m, series: option.Some(updated)),
      effect.none(),
    )
    PatchResponse(Error(_)) -> #(m, effect.none())
    OpenInteractiveSearch -> #(
      Model(..m, show_search_modal: True, release_search: option.None),
      series_api.find_releases(
        m.series_id,
        m.account.access_token,
        FindReleasesResponse,
      ),
    )
    CloseSearchModal -> #(Model(..m, show_search_modal: False), effect.none())
    FindReleasesResponse(Ok(rs)) -> {
      let poll_effect = case rs.status {
        "running" -> schedule_release_poll()
        _ -> effect.none()
      }
      #(Model(..m, release_search: option.Some(rs)), poll_effect)
    }
    FindReleasesResponse(Error(_)) -> #(m, effect.none())
    PollReleaseSearch ->
      case m.show_search_modal {
        True -> #(
          m,
          series_api.get_release_search(
            m.series_id,
            m.account.access_token,
            ReleaseSearchResponse,
          ),
        )
        False -> #(m, effect.none())
      }
    ReleaseSearchResponse(Ok(rs)) -> {
      let poll_effect = case rs.status, m.show_search_modal {
        "running", True -> schedule_release_poll()
        _, _ -> effect.none()
      }
      #(Model(..m, release_search: option.Some(rs)), poll_effect)
    }
    ReleaseSearchResponse(Error(_)) -> #(m, effect.none())
    GrabRelease(download_url, title) -> #(
      m,
      series_api.grab(
        m.series_id,
        download_url,
        title,
        m.account.access_token,
        fn(r) { GrabResponse(r, title) },
      ),
    )
    GrabResponse(Ok(_), title) -> #(
      Model(
        ..m,
        toast: option.Some(toast.Toast(
          toast.Success,
          "Sent \"" <> title <> "\" to qBittorrent",
        )),
      ),
      schedule_toast_dismiss(),
    )
    GrabResponse(Error(_), title) -> #(
      Model(
        ..m,
        toast: option.Some(toast.Toast(
          toast.Failure,
          "Failed to grab \"" <> title <> "\"",
        )),
      ),
      schedule_toast_dismiss(),
    )
    DismissToast -> #(Model(..m, toast: option.None), effect.none())
  }
}

fn view(m: Model) {
  html.div(
    [
      attribute.class(
        "min-h-full bg-zinc-950 text-white flex justify-center px-4 sm:px-8 py-4 sm:py-8",
      ),
    ],
    [
      modal.delete_modal(
        m.show_delete_modal,
        m.delete_files,
        ToggleDeleteFiles,
        CancelDelete,
        DeleteSeries,
      ),
      release_search_modal(m),
      case m.toast {
        option.None -> element.none()
        option.Some(t) -> toast.view(t)
      },
      html.div([attribute.class("w-full max-w-4xl")], [
        case m.loading {
          True -> display.loading()
          False ->
            case m.series {
              option.None ->
                html.div(
                  [
                    attribute.class(
                      "flex-1 flex flex-col items-center justify-center text-zinc-500",
                    ),
                  ],
                  [
                    element.text("Series not found."),
                  ],
                )
              option.Some(s) ->
                html.div([attribute.class("flex flex-col gap-6")], [
                  html.div([attribute.class("flex flex-col gap-2")], [
                    html.h1(
                      [attribute.class("text-2xl font-semibold truncate")],
                      [
                        element.text(s.title),
                      ],
                    ),
                    case s.alt_titles |> list.length {
                      0 -> element.none()
                      _ ->
                        html.h2([attribute.class("text-sm text-zinc-400")], [
                          element.text(
                            "aka: "
                            <> s.alt_titles
                            |> string.join(", "),
                          ),
                        ])
                    },
                  ]),
                  series_detail(s),
                ])
            }
        },
      ]),
    ],
  )
}

fn series_detail(s: series_api.Series) -> element.Element(Msg) {
  html.div([attribute.class("flex flex-col gap-6")], [
    html.div(
      [attribute.class("flex flex-col sm:flex-row items-start gap-4 sm:gap-6")],
      [
        case s.cover_url {
          "" ->
            html.div(
              [
                attribute.class(
                  "w-full sm:w-40 h-48 sm:h-56 bg-zinc-800 rounded-lg flex items-center justify-center",
                ),
              ],
              [
                html.i(
                  [attribute.class("ph ph-image text-zinc-600 text-3xl")],
                  [],
                ),
              ],
            )
          url ->
            html.img([
              attribute.src(url),
              attribute.alt(s.title),
              attribute.class(
                "w-full sm:w-40 h-48 sm:h-56 object-cover rounded-lg",
              ),
            ])
        },
        html.div([attribute.class("flex-1 flex flex-col gap-3")], [
          html.p([attribute.class("text-sm text-zinc-400")], [
            element.text(case s.year {
              0 -> s.status
              y -> s.status <> " · " <> int.to_string(y)
            }),
          ]),
          case s.overview {
            "" -> element.none()
            ov ->
              html.p(
                [attribute.class("text-sm text-zinc-500 leading-relaxed")],
                [
                  element.text(ov),
                ],
              )
          },
          html.div([attribute.class("flex flex-wrap items-center gap-2")], [
            button.icon_label(
              case s.monitored {
                True -> "ph-fill ph-bookmark-simple"
                False -> "ph ph-bookmark-simple"
              },
              case s.monitored {
                True -> "Monitored"
                False -> "Unmonitored"
              },
              [
                case s.monitored {
                  True -> button.primary()
                  False -> button.secondary()
                },
                event.on_click(ToggleSeriesMonitor(s.id, s.monitored)),
              ],
            ),
            button.icon_label("ph ph-arrow-clockwise", "Refresh Metadata", [
              button.secondary(),
              attribute.title("Refresh series metadata from source"),
              event.on_click(RefreshSeries),
            ]),
            button.icon_label("ph ph-magnifying-glass", "Search", [
              button.secondary(),
              attribute.title("Search for files"),
              event.on_click(SearchSeries),
            ]),
            button.icon_label(
              "ph ph-list-magnifying-glass",
              "Interactive Search",
              [
                button.secondary(),
                attribute.title("Browse releases and pick one manually"),
                event.on_click(OpenInteractiveSearch),
              ],
            ),
            button.icon_label("ph ph-trash", "Remove", [
              button.secondary(),
              attribute.title("Remove from library"),
              event.on_click(ShowDeleteModal),
            ]),
          ]),
          html.span([attribute.class("text-zinc-600 text-xs")], [
            element.text(case s.last_checked_at {
              "" -> "Not yet checked"
              t -> "Checked/Updated " <> t
            }),
          ]),
        ]),
      ],
    ),
    volume_section(s),
    chapter_section(s),
  ])
}

fn volume_section(s: series_api.Series) -> element.Element(Msg) {
  let volumes = case s.total_volumes {
    0 -> s.imported_volumes
    n -> int_range(1, n) |> list.map(int.to_string)
  }
  case volumes {
    [] -> element.none()
    _ ->
      html.div([attribute.class("flex flex-col gap-3")], [
        display.section_heading("Volumes", []),
        html.div(
          [attribute.class("flex flex-col divide-y divide-zinc-800")],
          list.map(volumes, fn(vol) {
            let is_imported = list.contains(s.imported_volumes, vol)
            let is_unmonitored = list.contains(s.unmonitored_volumes, vol)
            let #(status_label, status_attr) = case
              is_imported,
              is_unmonitored
            {
              True, _ -> #("Imported", badge.completed())
              False, True -> #("Unmonitored", badge.pending())
              False, False -> #("Missing", badge.warning())
            }
            html.div([attribute.class("flex items-center gap-3 py-2")], [
              button.icon(
                case is_unmonitored {
                  True -> "ph ph-bookmark-simple text-lg"
                  False -> "ph-fill ph-bookmark-simple text-lg"
                },
                [
                  button.ghost(),
                  attribute.class(case is_unmonitored {
                    True -> "text-zinc-600"
                    False -> "text-pink-400 hover:text-pink-300"
                  }),
                  attribute.title(case is_unmonitored {
                    True -> "Monitor"
                    False -> "Unmonitor"
                  }),
                  event.on_click(ToggleVolumeMonitor(s.id, vol, is_unmonitored)),
                ],
              ),
              html.span(
                [
                  attribute.class(
                    "text-sm flex-1 "
                    <> case is_unmonitored {
                      True -> "text-zinc-500"
                      False -> ""
                    },
                  ),
                ],
                [element.text("Vol " <> vol)],
              ),
              badge.badge(status_label, [status_attr]),
            ])
          }),
        ),
      ])
  }
}

fn chapter_section(s: series_api.Series) -> element.Element(Msg) {
  html.div([attribute.class("flex flex-col gap-3")], [
    html.div([attribute.class("flex items-center justify-between")], [
      display.section_heading("Chapters", []),
      button.icon_label(
        case s.monitor_chapters {
          True -> "ph-fill ph-bookmark-simple"
          False -> "ph ph-bookmark-simple"
        },
        case s.monitor_chapters {
          True -> "Monitoring"
          False -> "Ignored"
        },
        [
          attribute.class("text-xs py-1"),
          case s.monitor_chapters {
            True -> button.primary()
            False -> button.secondary()
          },
          event.on_click(ToggleChapterMonitor(s.id, s.monitor_chapters)),
        ],
      ),
    ]),
    case s.imported_chapters {
      [] ->
        html.p([attribute.class("text-sm text-zinc-600")], [
          element.text(
            "No chapters imported. Chapters and volumes cannot be cross-referenced.",
          ),
        ])
      chapters ->
        html.div(
          [attribute.class("flex flex-col divide-y divide-zinc-800")],
          list.map(chapters, fn(ch) {
            html.div([attribute.class("flex items-center gap-3 py-2")], [
              html.span([attribute.class("text-sm flex-1")], [
                element.text("Ch " <> ch),
              ]),
              badge.badge("Imported", [badge.completed()]),
            ])
          }),
        )
    },
  ])
}

fn format_size(bytes: Int) -> String {
  let kb = 1024
  let mb = kb * 1024
  let gb = mb * 1024
  case bytes {
    b if b >= gb -> int.to_string(b / gb) <> " GB"
    b if b >= mb -> int.to_string(b / mb) <> " MB"
    b if b >= kb -> int.to_string(b / kb) <> " KB"
    b -> int.to_string(b) <> " B"
  }
}

fn release_search_modal(m: Model) -> element.Element(Msg) {
  case m.show_search_modal {
    False -> element.none()
    True ->
      html.div(
        [
          attribute.class(
            "fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4",
          ),
        ],
        [
          html.div(
            [
              attribute.class(
                "bg-zinc-900 border border-zinc-800 rounded-xl w-full max-w-3xl max-h-[85vh] flex flex-col",
              ),
            ],
            [
              html.div(
                [
                  attribute.class(
                    "flex items-center justify-between px-6 py-4 border-b border-zinc-800 shrink-0",
                  ),
                ],
                [
                  html.h2([attribute.class("text-base font-semibold")], [
                    element.text("Interactive Search"),
                  ]),
                  button.icon("ph ph-x", [
                    button.ghost(),
                    event.on_click(CloseSearchModal),
                  ]),
                ],
              ),
              html.div([attribute.class("flex-1 overflow-auto px-6 py-4")], [
                release_search_body(m.release_search),
              ]),
            ],
          ),
        ],
      )
  }
}

fn release_search_body(
  release_search: option.Option(series_api.ReleaseSearch),
) -> element.Element(Msg) {
  case release_search {
    option.None ->
      html.div(
        [
          attribute.class(
            "flex flex-col items-center justify-center gap-3 py-12 text-zinc-500",
          ),
        ],
        [
          display.loading(),
          html.span([attribute.class("text-sm")], [
            element.text("Starting release search…"),
          ]),
        ],
      )
    option.Some(rs) ->
      case rs.status {
        "running" ->
          html.div(
            [
              attribute.class(
                "flex flex-col items-center justify-center gap-3 py-12 text-zinc-500",
              ),
            ],
            [
              display.loading(),
              html.span([attribute.class("text-sm")], [
                element.text("Searching for releases…"),
              ]),
            ],
          )
        "failed" ->
          html.p([attribute.class("text-sm text-red-400")], [
            element.text(case rs.error {
              "" -> "Search failed."
              e -> "Search failed: " <> e
            }),
          ])
        _ ->
          case rs.candidates {
            [] ->
              html.p([attribute.class("text-sm text-zinc-500")], [
                element.text("No releases found."),
              ])
            candidates ->
              html.div(
                [attribute.class("flex flex-col divide-y divide-zinc-800")],
                list.map(candidates, candidate_row),
              )
          }
      }
  }
}

fn candidate_row(c: series_api.Candidate) -> element.Element(Msg) {
  let #(status_label, status_attr) = case c.approved {
    True -> #("Approved", badge.completed())
    False -> #("Rejected", badge.failed())
  }
  html.div([attribute.class("flex items-center gap-3 py-3")], [
    html.div([attribute.class("flex-1 min-w-0 flex flex-col gap-1")], [
      html.div([attribute.class("flex items-center gap-2 min-w-0")], [
        case c.approved, c.reject_reason {
          False, reason if reason != "" ->
            html.i(
              [
                attribute.class("ph-fill ph-warning text-red-400 shrink-0"),
                attribute.title(reason),
              ],
              [],
            )
          _, _ -> element.none()
        },
        html.span([attribute.class("text-sm truncate")], [
          element.text(c.title),
        ]),
      ]),
      html.span([attribute.class("text-xs text-zinc-500")], [
        element.text(
          c.indexer
          <> " · "
          <> int.to_string(c.seeders)
          <> "/"
          <> int.to_string(c.leechers)
          <> " peers · "
          <> format_size(c.size),
        ),
      ]),
    ]),
    badge.badge(status_label, [status_attr]),
    button.icon("ph ph-download-simple", [
      button.ghost(),
      attribute.title("Send to client"),
      event.on_click(GrabRelease(c.download_url, c.title)),
    ]),
  ])
}
