import gleam/json
import gleam/list
import gleam/string
import lustre
import lustre/attribute
import lustre/effect
import lustre/element
import lustre/element/html
import plinth/javascript/storage
import rsvp
import tankobon/api/auth
import tankobon/api/jobs as imports_api
import tankobon/ui/display

pub type Model {
  Model(
    account: auth.Account,
    history: List(imports_api.ImportLog),
    loading: Bool,
  )
}

pub type Msg {
  Load
  HistoryResponse(Result(List(imports_api.ImportLog), rsvp.Error(String)))
}

pub fn register() {
  let app = lustre.component(init, update, view, [])
  lustre.register(app, "jobs-page")
}

pub fn element() {
  element.element("jobs-page", [], [])
}

fn init(_) {
  let assert Ok(stg) = storage.local()
  let assert Ok(account_json) = stg |> storage.get_item("tankobon_account")
  let assert Ok(account) = json.parse(account_json, auth.account_decoder())
  #(
    Model(account:, history: [], loading: True),
    imports_api.get_history(account.access_token, HistoryResponse),
  )
}

fn update(m: Model, msg: Msg) {
  case msg {
    Load -> #(
      Model(..m, loading: True),
      imports_api.get_history(m.account.access_token, HistoryResponse),
    )
    HistoryResponse(Ok(history)) -> #(
      Model(..m, history:, loading: False),
      effect.none(),
    )
    HistoryResponse(Error(_)) -> #(Model(..m, loading: False), effect.none())
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
      html.div([attribute.class("w-full max-w-4xl flex flex-col gap-8")], [
        case m.loading {
          True -> display.loading()
          False ->
            case m.history {
              [] ->
                html.div(
                  [
                    attribute.class(
                      "flex-1 flex flex-col items-center justify-center gap-2 text-zinc-500 text-center",
                    ),
                  ],
                  [
                    html.p([attribute.class("text-lg")], [
                      element.text("No imports yet."),
                    ]),
                    html.p([attribute.class("text-sm")], [
                      element.text("Completed imports will appear here."),
                    ]),
                  ],
                )
              entries ->
                html.div(
                  [attribute.class("flex flex-col gap-2")],
                  list.map(entries, history_row),
                )
            }
        },
      ]),
    ],
  )
}

fn history_row(entry: imports_api.ImportLog) {
  let date = entry.created_at
  let content_summary = case entry.volumes, entry.chapters {
    [], [] -> "No content"
    vols, [] ->
      case list.length(vols) {
        1 -> "Vol " <> string.join(vols, ", ")
        n -> string.inspect(n) <> " volumes"
      }
    [], chs ->
      case list.length(chs) {
        1 -> "Ch " <> string.join(chs, ", ")
        n -> string.inspect(n) <> " chapters"
      }
    vols, chs ->
      string.inspect(list.length(vols))
      <> "v · "
      <> string.inspect(list.length(chs))
      <> "ch"
  }

  html.div(
    [
      attribute.class(
        "flex items-center gap-4 px-4 py-3 bg-zinc-900 rounded-lg",
      ),
    ],
    [
      html.div([attribute.class("flex-1 min-w-0 flex flex-col gap-0.5")], [
        html.p([attribute.class("font-medium text-sm truncate")], [
          element.text(entry.series_title),
        ]),
        html.p([attribute.class("text-xs text-zinc-500 truncate")], [
          element.text(entry.torrent_name),
        ]),
      ]),
      html.div(
        [attribute.class("text-right flex-shrink-0 flex flex-col gap-0.5")],
        [
          html.p([attribute.class("text-xs text-zinc-400")], [
            element.text(content_summary),
          ]),
          html.p([attribute.class("text-xs text-zinc-600")], [
            element.text(date),
          ]),
        ],
      ),
    ],
  )
}
