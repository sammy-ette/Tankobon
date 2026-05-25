import gleam/int
import gleam/json
import gleam/list
import lustre
import lustre/attribute
import lustre/effect
import lustre/element
import lustre/element/html
import lustre/event
import plinth/javascript/storage
import rsvp
import tankobon/api/auth
import tankobon/api/series as series_api
import tankobon/ui/button
import tankobon/ui/display
import tankobon/ui/modal

pub type Model {
  Model(
    account: auth.Account,
    series: List(series_api.Series),
    loading: Bool,
    show_delete_modal: Bool,
    delete_files: Bool,
    pending_delete_id: Int,
  )
}

pub type Msg {
  LoadSeries
  SeriesResponse(Result(List(series_api.Series), rsvp.Error(String)))
  ShowDeleteModal(Int)
  ToggleDeleteFiles
  CancelDelete
  ConfirmDelete
  DeleteSeriesResponse(Result(Nil, rsvp.Error(String)))
}

pub fn register() {
  let app = lustre.component(init, update, view, [])
  lustre.register(app, "home-page")
}

pub fn element() {
  element.element("home-page", [], [])
}

fn init(_) {
  let assert Ok(stg) = storage.local()
  let assert Ok(account_json) = stg |> storage.get_item("tankobon_account")
  let assert Ok(account) = json.parse(account_json, auth.account_decoder())
  #(
    Model(
      account:,
      series: [],
      loading: True,
      show_delete_modal: False,
      delete_files: False,
      pending_delete_id: 0,
    ),
    series_api.list(account.access_token, SeriesResponse),
  )
}

fn update(m: Model, msg: Msg) {
  case msg {
    LoadSeries -> #(
      Model(..m, loading: True),
      series_api.list(m.account.access_token, SeriesResponse),
    )
    SeriesResponse(Ok(series)) -> #(
      Model(..m, series:, loading: False),
      effect.none(),
    )
    SeriesResponse(Error(err)) -> {
      echo err
      #(Model(..m, loading: False), effect.none())
    }
    ShowDeleteModal(id) -> #(
      Model(
        ..m,
        show_delete_modal: True,
        delete_files: False,
        pending_delete_id: id,
      ),
      effect.none(),
    )
    ToggleDeleteFiles -> #(
      Model(..m, delete_files: !m.delete_files),
      effect.none(),
    )
    CancelDelete -> #(Model(..m, show_delete_modal: False), effect.none())
    ConfirmDelete -> #(
      Model(..m, show_delete_modal: False),
      series_api.delete(
        m.pending_delete_id,
        m.account.access_token,
        m.delete_files,
        DeleteSeriesResponse,
      ),
    )
    DeleteSeriesResponse(Ok(_)) -> #(
      Model(..m, loading: True),
      series_api.list(m.account.access_token, SeriesResponse),
    )
    DeleteSeriesResponse(Error(_)) -> #(m, effect.none())
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
        ConfirmDelete,
      ),
      html.div([attribute.class("w-full flex flex-col")], [
        case m.loading {
          True -> display.loading()
          False ->
            case m.series {
              [] ->
                html.div(
                  [
                    attribute.class(
                      "flex-1 flex flex-col items-center justify-center gap-2 text-zinc-500 text-center",
                    ),
                  ],
                  [
                    html.p([attribute.class("text-lg")], [
                      element.text("No series in your library yet."),
                    ]),
                    html.p([attribute.class("text-sm")], [
                      element.text(
                        "Search for manga and add them to get started.",
                      ),
                    ]),
                  ],
                )
              series ->
                html.div(
                  [
                    attribute.class(
                      "grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7 xl:grid-cols-8 gap-8",
                    ),
                  ],
                  list.map(series, series_card),
                )
            }
        },
      ]),
    ],
  )
}

fn series_card(s: series_api.Series) -> element.Element(Msg) {
  let missing = s.total_volumes - list.length(s.imported_volumes)
  let status_text = case s.year {
    0 -> s.status
    y ->
      s.status
      <> " · "
      <> int.to_string(y)
      <> case missing {
        0 -> ""
        _ -> " · " <> int.to_string(missing) <> " missing"
      }
  }
  html.div([attribute.class("relative")], [
    html.a(
      [
        attribute.href("/series/" <> int.to_string(s.id)),
        attribute.class("flex flex-col gap-2"),
      ],
      [
        html.div(
          [
            attribute.class(
              "relative aspect-[2/3] rounded-lg overflow-hidden bg-zinc-800",
            ),
          ],
          [
            case s.cover_url {
              "" ->
                html.div(
                  [
                    attribute.class(
                      "w-full h-full flex items-center justify-center text-zinc-600",
                    ),
                  ],
                  [html.i([attribute.class("ph ph-image text-3xl")], [])],
                )
              url ->
                html.div([attribute.class("w-full h-full")], [
                  html.img([
                    attribute.src(url),
                    attribute.alt(s.title),
                    attribute.class("w-full h-full object-cover"),
                  ]),
                  case s.monitored {
                    True -> element.none()
                    False ->
                      html.div(
                        [attribute.class("absolute inset-0 bg-zinc-950/50")],
                        [],
                      )
                  },
                ])
            },
          ],
        ),
        html.div([attribute.class("flex flex-col gap-0.5")], [
          html.p(
            [
              attribute.class("text-xs font-medium leading-tight line-clamp-2"),
              case s.monitored {
                True -> attribute.none()
                False -> attribute.class("text-zinc-400")
              },
            ],
            [element.text(s.title)],
          ),
          html.p([attribute.class("text-xs text-zinc-600 hidden sm:block")], [
            element.text(status_text),
          ]),
        ]),
      ],
    ),
    button.icon("ph ph-trash text-sm", [
      attribute.class(
        "absolute top-1.5 right-1.5 p-1.5 rounded-md bg-black/50 text-zinc-400 hover:text-red-400 hover:bg-black/70",
      ),
      attribute.title("Remove from library"),
      event.on_click(ShowDeleteModal(s.id)),
    ]),
  ])
}
