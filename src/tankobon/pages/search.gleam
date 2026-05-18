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
import tankobon/ui/input

pub type Model {
  Model(
    account: auth.Account,
    query: String,
    loading: Bool,
    results: List(series_api.SearchResult),
    added: List(Int),
  )
}

pub type Msg {
  QueryChanged(String)
  SubmitSearch
  SearchResponse(Result(List(series_api.SearchResult), rsvp.Error(String)))
  AddToLibrary(Int)
  AddResponse(Result(Int, rsvp.Error(String)))
}

pub fn register() {
  let app = lustre.component(init, update, view, [])
  lustre.register(app, "search-page")
}

pub fn element() {
  element.element("search-page", [], [])
}

fn init(_) {
  let assert Ok(stg) = storage.local()
  let assert Ok(account_json) = stg |> storage.get_item("tankobon_account")
  let assert Ok(account) = json.parse(account_json, auth.account_decoder())
  #(
    Model(account:, query: "", loading: False, results: [], added: []),
    effect.none(),
  )
}

fn update(m: Model, msg: Msg) {
  case msg {
    QueryChanged(q) -> #(Model(..m, query: q), effect.none())
    SubmitSearch ->
      case m.query {
        "" -> #(m, effect.none())
        q -> #(
          Model(..m, loading: True, results: []),
          series_api.search(q, m.account.access_token, SearchResponse),
        )
      }
    SearchResponse(Ok(results)) -> #(
      Model(..m, loading: False, results: results),
      effect.none(),
    )
    SearchResponse(Error(err)) -> {
      echo err
      #(Model(..m, loading: False), effect.none())
    }
    AddToLibrary(id) -> #(
      m,
      series_api.add(id, m.account.access_token, AddResponse),
    )
    AddResponse(Ok(id)) -> #(Model(..m, added: [id, ..m.added]), effect.none())
    AddResponse(Error(_)) -> #(m, effect.none())
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
      html.div([attribute.class("w-full max-w-4xl flex flex-col gap-6")], [
        html.form(
          [
            attribute.class("flex gap-2"),
            event.prevent_default(event.on_submit(fn(_) { SubmitSearch })),
          ],
          [
            input.input([
              attribute.type_("text"),
              attribute.placeholder("Search manga..."),
              attribute.value(m.query),
              attribute.class("flex-1 min-w-0 py-2"),
              attribute.disabled(m.loading),
              event.on_input(QueryChanged),
            ]),
            button.button(
              case m.loading {
                True -> "Searching..."
                False -> "Search"
              },
              [
                button.primary(),
                attribute.class(
                  "shrink-0 whitespace-nowrap disabled:bg-zinc-700",
                ),
                attribute.disabled(m.loading),
              ],
            ),
          ],
        ),
        case m.loading {
          True ->
            html.div([attribute.class("text-zinc-500 text-sm")], [
              element.text("Searching..."),
            ])
          False ->
            html.div(
              [attribute.class("flex flex-col gap-3")],
              list.map(m.results, fn(r) {
                result_row(r, list.contains(m.added, r.id))
              }),
            )
        },
      ]),
    ],
  )
}

fn result_row(
  r: series_api.SearchResult,
  already_added: Bool,
) -> element.Element(Msg) {
  html.div(
    [attribute.class("flex gap-3 sm:gap-4 p-3 sm:p-4 bg-zinc-900 rounded-lg")],
    [
      case r.cover_url {
        "" ->
          html.div(
            [
              attribute.class(
                "w-20 sm:w-32 h-28 sm:h-44 bg-zinc-800 rounded flex items-center justify-center flex-shrink-0",
              ),
            ],
            [html.i([attribute.class("ph ph-image text-zinc-600")], [])],
          )
        url ->
          html.img([
            attribute.src(url),
            attribute.alt(r.title),
            attribute.class(
              "w-20 sm:w-32 h-28 sm:h-44 object-cover rounded flex-shrink-0",
            ),
          ])
      },
      html.div([attribute.class("flex flex-col gap-1 min-w-0")], [
        html.p([attribute.class("font-medium")], [
          element.text(r.title),
        ]),
        html.p([attribute.class("text-sm text-zinc-400")], [
          element.text(case r.year {
            0 -> r.status
            y -> r.status <> " · " <> int.to_string(y)
          }),
        ]),
        case r.overview {
          "" -> element.none()
          overview ->
            html.p(
              [attribute.class("text-xs text-zinc-500 line-clamp-2")],
              [
                element.text(overview),
              ],
            )
        },

        case already_added {
          True ->
            html.span(
              [attribute.class("self-start px-4 py-2 text-sm text-zinc-500")],
              [
                element.text("Added"),
              ],
            )
          False ->
            button.button("Add to Library", [
              button.primary(),
              attribute.class("self-start"),
              event.on_click(AddToLibrary(r.id)),
            ])
        },
      ]),
    ],
  )
}
