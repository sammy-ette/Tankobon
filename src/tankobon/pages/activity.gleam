import gleam/bool
import gleam/dict
import gleam/float
import gleam/int
import gleam/json
import gleam/list
import gleam/option
import gleam/result
import gleam/string
import lustre
import lustre/attribute
import lustre/effect
import lustre/element
import lustre/element/html
import lustre/event
import plinth/javascript/storage
import rsvp
import tankobon/api/activity as activity_api
import tankobon/api/auth
import tankobon/api/jobs as imports_api
import tankobon/api/series as series_api
import tankobon/ui/badge
import tankobon/ui/button
import tankobon/ui/display
import tankobon/ui/input

pub type MappingState {
  MappingState(
    hash: String,
    series_id: Int,
    loading: Bool,
    files: List(imports_api.FileEntry),
    volumes: dict.Dict(String, String),
    chapters: dict.Dict(String, String),
    checked: dict.Dict(String, Bool),
  )
}

pub type Model {
  Model(
    account: auth.Account,
    activity: List(activity_api.ActivityItem),
    imports: List(imports_api.PendingImport),
    series: List(series_api.Series),
    selected: dict.Dict(String, Int),
    loading: Bool,
    mapping: option.Option(MappingState),
  )
}

pub type Msg {
  Load
  ActivityResponse(Result(List(activity_api.ActivityItem), rsvp.Error(String)))
  ImportsResponse(Result(List(imports_api.PendingImport), rsvp.Error(String)))
  SeriesLoaded(Result(List(series_api.Series), rsvp.Error(String)))
  SelectSeries(String, Int)
  OpenMapping(String, Int)
  CloseMapping
  FilesLoaded(Result(List(imports_api.FileEntry), rsvp.Error(String)))
  ToggleFile(String)
  ToggleAll(Bool)
  SetFileVolume(String, String)
  SetFileChapter(String, String)
  ImportSeries
  ImportSeriesResponse(Result(Nil, rsvp.Error(String)))
}

pub fn register() {
  let app = lustre.component(init, update, view, [])
  lustre.register(app, "activity-page")
}

pub fn element() {
  element.element("activity-page", [], [])
}

fn init(_) {
  let assert Ok(stg) = storage.local()
  let assert Ok(account_json) = stg |> storage.get_item("tankobon_account")
  let assert Ok(account) = json.parse(account_json, auth.account_decoder())
  #(
    Model(
      account:,
      activity: [],
      imports: [],
      series: [],
      selected: dict.new(),
      loading: True,
      mapping: option.None,
    ),
    effect.batch([
      activity_api.get_activity(account.access_token, ActivityResponse),
      imports_api.list_imports(account.access_token, ImportsResponse),
      series_api.list(account.access_token, SeriesLoaded),
    ]),
  )
}

fn reload(m: Model) -> effect.Effect(Msg) {
  effect.batch([
    activity_api.get_activity(m.account.access_token, ActivityResponse),
    imports_api.list_imports(m.account.access_token, ImportsResponse),
  ])
}

fn update(m: Model, msg: Msg) {
  case msg {
    Load -> #(Model(..m, loading: True), reload(m))
    ActivityResponse(Ok(activity)) -> #(
      Model(..m, activity:, loading: False),
      effect.none(),
    )
    ActivityResponse(Error(_)) -> #(Model(..m, loading: False), effect.none())
    ImportsResponse(Ok(imports)) -> #(
      Model(..m, imports:, loading: False),
      effect.none(),
    )
    ImportsResponse(Error(_)) -> #(Model(..m, loading: False), effect.none())
    SeriesLoaded(Ok(series)) -> #(Model(..m, series:), effect.none())
    SeriesLoaded(Error(_)) -> #(m, effect.none())
    SelectSeries(hash, series_id) -> #(
      Model(..m, selected: case series_id > 0 {
        True -> dict.insert(m.selected, hash, series_id)
        False -> dict.delete(m.selected, hash)
      }),
      effect.none(),
    )
    OpenMapping(hash, series_id) -> {
      let state =
        MappingState(
          hash:,
          series_id:,
          loading: True,
          files: [],
          volumes: dict.new(),
          chapters: dict.new(),
          checked: dict.new(),
        )
      #(
        Model(..m, mapping: option.Some(state)),
        imports_api.get_import_files(hash, m.account.access_token, FilesLoaded),
      )
    }

    CloseMapping -> #(Model(..m, mapping: option.None), effect.none())

    FilesLoaded(Ok(files)) ->
      case m.mapping {
        option.None -> #(m, effect.none())
        option.Some(state) -> {
          let volumes =
            list.fold(files, dict.new(), fn(acc, f) {
              case string.join(f.volumes, ",") {
                "" -> acc
                v -> dict.insert(acc, f.path, v)
              }
            })
          let chapters =
            list.fold(files, dict.new(), fn(acc, f) {
              case string.join(f.chapters, ",") {
                "" -> acc
                c -> dict.insert(acc, f.path, c)
              }
            })
          let checked =
            list.fold(files, dict.new(), fn(acc, f) {
              dict.insert(acc, f.path, True)
            })
          #(
            Model(
              ..m,
              mapping: option.Some(
                MappingState(
                  ..state,
                  loading: False,
                  files:,
                  volumes:,
                  chapters:,
                  checked:,
                ),
              ),
            ),
            effect.none(),
          )
        }
      }

    FilesLoaded(Error(_)) ->
      case m.mapping {
        option.None -> #(m, effect.none())
        option.Some(state) -> #(
          Model(
            ..m,
            mapping: option.Some(MappingState(..state, loading: False)),
          ),
          effect.none(),
        )
      }

    ToggleFile(path) ->
      update_mapping(m, fn(state) {
        MappingState(
          ..state,
          checked: dict.insert(
            state.checked,
            path,
            dict.get(state.checked, path) != Ok(True),
          ),
        )
      })

    ToggleAll(value) ->
      update_mapping(m, fn(state) {
        MappingState(
          ..state,
          checked: list.fold(state.files, dict.new(), fn(acc, f) {
            dict.insert(acc, f.path, value)
          }),
        )
      })

    SetFileVolume(path, value) ->
      update_mapping(m, fn(state) {
        MappingState(..state, volumes: dict.insert(state.volumes, path, value))
      })

    SetFileChapter(path, value) ->
      update_mapping(m, fn(state) {
        MappingState(
          ..state,
          chapters: dict.insert(state.chapters, path, value),
        )
      })

    ImportSeries ->
      case m.mapping {
        option.None -> #(m, effect.none())
        option.Some(state) -> {
          let file_mappings =
            list.filter_map(state.files, fn(f) {
              use <- bool.guard(
                dict.get(state.checked, f.path) != Ok(True),
                Error(Nil),
              )
              let vols =
                dict.get(state.volumes, f.path)
                |> result.unwrap("")
                |> string.split(",")
                |> list.map(string.trim)
                |> list.filter(fn(s) { s != "" })
              let chaps =
                dict.get(state.chapters, f.path)
                |> result.unwrap("")
                |> string.split(",")
                |> list.map(string.trim)
                |> list.filter(fn(s) { s != "" })
              case vols, chaps {
                [], [] -> Error(Nil)
                _, _ ->
                  Ok(imports_api.FileMapping(
                    path: f.path,
                    volumes: vols,
                    chapters: chaps,
                  ))
              }
            })
          #(
            m,
            imports_api.manual_import(
              state.hash,
              state.series_id,
              file_mappings,
              m.account.access_token,
              ImportSeriesResponse,
            ),
          )
        }
      }

    ImportSeriesResponse(Ok(_)) -> #(
      Model(..m, mapping: option.None, loading: True),
      reload(m),
    )
    ImportSeriesResponse(Error(_)) -> #(m, effect.none())
  }
}

fn update_mapping(
  m: Model,
  f: fn(MappingState) -> MappingState,
) -> #(Model, effect.Effect(Msg)) {
  case m.mapping {
    option.None -> #(m, effect.none())
    option.Some(state) -> #(
      Model(..m, mapping: option.Some(f(state))),
      effect.none(),
    )
  }
}

fn view(m: Model) {
  case m.mapping {
    option.Some(state) -> mapping_view(state, m.imports, m.series)
    option.None -> queue_view(m)
  }
}

fn queue_view(m: Model) {
  html.div(
    [
      attribute.class(
        "min-h-full bg-zinc-950 text-white flex justify-center px-4 sm:px-8 py-4 sm:py-8",
      ),
    ],
    [
      html.div([attribute.class("w-full max-w-2/3 flex flex-col gap-8")], [
        case m.loading {
          True -> display.loading()
          False ->
            html.div([attribute.class("flex-1 flex flex-col gap-8")], [
              case
                list.filter(m.activity, fn(i) { i.processing |> bool.negate })
              {
                [] -> element.none()
                items ->
                  html.div([attribute.class("flex flex-col gap-3")], [
                    display.section_heading("Downloading", []),
                    html.div(
                      [attribute.class("flex flex-col gap-3")],
                      list.map(items, activity_card),
                    ),
                  ])
              },
              case list.filter(m.activity, fn(i) { i.processing }) {
                [] -> element.none()
                items ->
                  html.div([attribute.class("flex flex-col gap-3")], [
                    display.section_heading("Processing", []),
                    html.div(
                      [attribute.class("flex flex-col gap-3")],
                      list.map(items, activity_card),
                    ),
                  ])
              },
              case m.imports {
                [] -> element.none()
                imports ->
                  html.div([attribute.class("flex flex-col gap-3")], [
                    display.section_heading("Needs Attention", []),
                    html.div(
                      [attribute.class("flex flex-col gap-3")],
                      list.map(imports, fn(i) {
                        import_card(i, m.series, m.selected)
                      }),
                    ),
                  ])
              },
              case m.activity, m.imports {
                [], [] ->
                  html.div(
                    [
                      attribute.class(
                        "flex-1 flex flex-col items-center justify-center gap-2 text-zinc-500 text-center",
                      ),
                    ],
                    [
                      html.p([attribute.class("text-lg")], [
                        element.text("Queue is empty."),
                      ]),
                      html.p([attribute.class("text-sm")], [
                        element.text(
                          "Active downloads and pending imports will appear here.",
                        ),
                      ]),
                    ],
                  )
                _, _ -> element.none()
              },
            ])
        },
      ]),
    ],
  )
}

fn torrent_card(name, subtitle, badge, body) {
  html.div([attribute.class("bg-zinc-900 rounded-lg p-4 flex flex-col gap-3")], [
    html.div([attribute.class("flex justify-between items-start gap-4")], [
      html.div([attribute.class("min-w-0 flex-1 flex flex-col gap-0.5")], [
        html.p([attribute.class("font-medium truncate text-sm")], [
          element.text(name),
        ]),
        subtitle,
      ]),
      badge,
    ]),
    body,
  ])
}

fn activity_card(item: activity_api.ActivityItem) {
  let progress = int.to_string(float.round(item.torrent.progress))
  let body = case item.processing {
    True -> element.none()
    False ->
      html.div([attribute.class("flex flex-col gap-1")], [
        html.div([attribute.class("text-xs text-zinc-500")], [
          element.text(progress <> "%"),
        ]),
        html.div([attribute.class("w-full bg-zinc-800 rounded-full h-1")], [
          html.div(
            [
              attribute.class("bg-violet-500 h-1 rounded-full transition-all"),
              attribute.style("width", progress <> "%"),
            ],
            [],
          ),
        ]),
      ])
  }
  let badge = case item.processing {
    True -> badge.badge("Processing", [badge.processing()])
    False ->
      case item.torrent.state {
        "downloading" | "allocating" | "checkingDL" | "forcedDL" ->
          badge.badge("Downloading", [badge.running()])
        "uploading" | "forcedUP" | "stalledUP" | "queuedUP" | "pausedUP" ->
          badge.badge("Seeding", [badge.completed()])
        "stoppedDL" | "metaDL" | "stalledDL" | "queuedDL" | "pausedDL" ->
          badge.badge("Queued", [badge.pending()])
        "error" | "missingFiles" -> badge.badge("Error", [badge.failed()])
        state -> badge.badge(state, [badge.pending()])
      }
  }
  torrent_card(item.torrent.name, element.none(), badge, body)
}

fn import_card(
  i: imports_api.PendingImport,
  series: List(series_api.Series),
  selected: dict.Dict(String, Int),
) {
  let hash = i.torrent.hash
  let subtitle = case i.series {
    option.Some(s) ->
      html.p(
        [attribute.class("text-xs text-zinc-500 flex items-center gap-2")],
        [
          html.span([], [element.text(s.title)]),
          case i.shape {
            "" -> element.none()
            shape ->
              html.span([attribute.class("opacity-60")], [
                element.text(
                  "· "
                  <> case shape {
                    "volume_only" -> "Volumes"
                    "chapter_only" -> "Chapters"
                    "mixed" -> "Mixed"
                    _ -> shape
                  },
                ),
              ])
          },
        ],
      )
    option.None -> element.none()
  }
  torrent_card(
    i.torrent.name,
    subtitle,
    reason_badge(i.reason),
    series_selector(hash, series, selected),
  )
}

fn series_selector(
  hash: String,
  series: List(series_api.Series),
  selected: dict.Dict(String, Int),
) {
  let current_id = dict.get(selected, hash) |> option.from_result
  html.div([attribute.class("flex flex-wrap gap-2")], [
    html.select(
      [
        attribute.class(
          "flex-1 bg-zinc-800 rounded px-2 py-1.5 text-sm text-white border border-zinc-700 focus:outline-none focus:border-violet-500",
        ),
        event.on_change(fn(val) {
          let series_id = int.parse(val) |> result.unwrap(0)
          SelectSeries(hash, series_id)
        }),
      ],
      [
        html.option(
          [attribute.value(""), attribute.selected(option.is_none(current_id))],
          "Select series…",
        ),
        ..list.map(series, fn(s) {
          html.option(
            [
              attribute.value(int.to_string(s.id)),
              attribute.selected(current_id == option.Some(s.id)),
            ],
            s.title,
          )
        })
      ],
    ),
    case current_id {
      option.Some(series_id) ->
        button.button("Import", [
          button.primary(),
          attribute.class("text-xs py-1.5 whitespace-nowrap"),
          event.on_click(OpenMapping(hash, series_id)),
        ])
      option.None ->
        button.button("Import", [
          button.secondary(),
          attribute.class("text-xs py-1.5 whitespace-nowrap"),
          attribute.disabled(True),
        ])
    },
  ])
}

fn reason_badge(reason: imports_api.PendingReason) {
  case reason {
    imports_api.Unmatched -> badge.badge("Unmatched", [badge.warning()])
    imports_api.NoFiles -> badge.badge("No Archive Files", [badge.failed()])
    imports_api.NeedsReview -> badge.badge("Needs Review", [badge.warning()])
    imports_api.Unknown(reason) ->
      badge.badge("Unknown (" <> reason <> ")", [badge.pending()])
  }
}

fn mapping_view(
  state: MappingState,
  imports: List(imports_api.PendingImport),
  series: List(series_api.Series),
) {
  let torrent_name =
    list.find(imports, fn(i) { i.torrent.hash == state.hash })
    |> result.map(fn(i) { i.torrent.name })
    |> result.unwrap("Unknown Torrent")

  let series_name =
    list.find(series, fn(s) { s.id == state.series_id })
    |> result.map(fn(s) { s.title })
    |> result.unwrap("Unknown Series")

  html.div(
    [
      attribute.class(
        "min-h-full bg-zinc-950 text-white px-4 sm:px-8 py-4 sm:py-8",
      ),
    ],
    [
      html.div(
        [attribute.class("w-full max-w-4xl mx-auto flex flex-col gap-6")],
        [
          html.button(
            [
              attribute.class(
                "self-start text-sm text-zinc-400 hover:text-white transition-colors",
              ),
              event.on_click(CloseMapping),
            ],
            [element.text("← Back")],
          ),
          html.div([attribute.class("flex flex-col gap-0.5")], [
            html.h1([attribute.class("text-xl font-semibold truncate")], [
              element.text(torrent_name),
            ]),
            html.p([attribute.class("text-sm text-zinc-400")], [
              element.text("Importing as: " <> series_name),
            ]),
          ]),
          case state.loading {
            True ->
              html.div([attribute.class("text-zinc-500 text-sm")], [
                element.text("Loading files..."),
              ])
            False ->
              case state.files {
                [] ->
                  html.div([attribute.class("text-zinc-500 text-sm")], [
                    element.text("No archive files found in this torrent."),
                  ])
                files ->
                  html.div([attribute.class("flex flex-col gap-4")], [
                    html.div([attribute.class("flex items-center gap-2 px-3")], [
                      input.checkbox([
                        attribute.checked(
                          list.all(files, fn(f) {
                            dict.get(state.checked, f.path) == Ok(True)
                          }),
                        ),
                        event.on_check(ToggleAll),
                      ]),
                      html.span([attribute.class("text-xs text-zinc-400")], [
                        element.text("Select all"),
                      ]),
                    ]),
                    html.div(
                      [attribute.class("flex flex-col gap-1")],
                      list.map(files, fn(f) { file_row(f, state) }),
                    ),
                    button.button("Import", [
                      button.primary(),
                      attribute.class("w-full py-2.5"),
                      event.on_click(ImportSeries),
                    ]),
                  ])
              }
          },
        ],
      ),
    ],
  )
}

fn file_row(f: imports_api.FileEntry, state: MappingState) {
  let is_checked = dict.get(state.checked, f.path) == Ok(True)
  let vol_val = dict.get(state.volumes, f.path) |> result.unwrap("")
  let ch_val = dict.get(state.chapters, f.path) |> result.unwrap("")
  let filename =
    f.path |> string.split("/") |> list.last() |> result.unwrap(f.path)

  html.div(
    [
      attribute.class(
        "flex items-center gap-3 rounded-lg px-3 py-2 "
        <> case is_checked {
          True -> "bg-zinc-900"
          False -> "bg-zinc-900/50 opacity-60"
        },
      ),
    ],
    [
      input.checkbox([
        attribute.checked(is_checked),
        attribute.class("flex-shrink-0"),
        event.on_click(ToggleFile(f.path)),
      ]),
      html.div(
        [
          attribute.class(
            "overflow-x-auto flex-1 min-w-0 [&::-webkit-scrollbar]:h-1 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-zinc-600 [&::-webkit-scrollbar-thumb]:rounded-full",
          ),
        ],
        [
          html.span(
            [
              attribute.class("text-sm font-mono whitespace-nowrap block"),
            ],
            [element.text(filename)],
          ),
        ],
      ),
      html.div([attribute.class("flex items-center gap-2 flex-shrink-0")], [
        html.span([attribute.class("text-xs text-zinc-500 w-5 text-right")], [
          element.text("Vol"),
        ]),
        input.input([
          attribute.class("w-16 text-xs text-center"),
          attribute.value(vol_val),
          attribute.placeholder("—"),
          event.on_input(fn(v) { SetFileVolume(f.path, v) }),
        ]),
        html.span([attribute.class("text-xs text-zinc-500 w-5 text-right")], [
          element.text("Ch"),
        ]),
        input.input([
          attribute.class("w-16 text-xs text-center"),
          attribute.value(ch_val),
          attribute.placeholder("—"),
          event.on_input(fn(v) { SetFileChapter(f.path, v) }),
        ]),
      ]),
    ],
  )
}
