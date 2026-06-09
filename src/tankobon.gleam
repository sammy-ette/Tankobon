import gleam/json
import gleam/option
import gleam/uri
import lustre
import lustre/attribute
import lustre/effect
import lustre/element
import lustre/element/html
import lustre/event
import modem
import plinth/javascript/date
import plinth/javascript/global
import plinth/javascript/storage
import rsvp
import tankobon/api/activity as activity_api
import tankobon/api/auth
import tankobon/api/series as series_api
import tankobon/api/status as status_api
import tankobon/pages/activity
import tankobon/pages/config
import tankobon/pages/elements
import tankobon/pages/home
import tankobon/pages/imports
import tankobon/pages/login
import tankobon/pages/register
import tankobon/pages/search
import tankobon/pages/series
import tankobon/router
import tankobon/ui/button

const refresh_interval_ms = 600_000

type Model {
  Model(
    route: router.Route,
    account: option.Option(auth.Account),
    mobile_menu_open: Bool,
    last_refreshed_at: option.Option(Int),
    finished: Bool,
    status: status_api.Status,
  )
}

pub type Msg {
  ChangeRoute(router.Route)
  ToggleMenu
  Logout
  RefreshAuth
  RefreshAuthResponse(Result(auth.Account, rsvp.Error(String)))
  Scan
  ScanResponse(Result(Nil, rsvp.Error(String)))
  RefreshAllMetadata
  RefreshAllMetadataResponse(Result(Nil, rsvp.Error(String)))
  ExistsResponse(Result(Bool, rsvp.Error(String)))
  PollStatus
  StatusResponse(Result(status_api.Status, rsvp.Error(String)))
}

const status_poll_interval_ms = 10_000

pub fn main() {
  let app = lustre.application(init, update, view)
  let assert Ok(_) = login.register()
  let assert Ok(_) = register.register()
  let assert Ok(stg) = storage.local()
  case stg |> storage.get_item("tankobon_account") {
    Ok(json) ->
      case json.parse(json, auth.account_decoder()) {
        Ok(_) -> {
          let assert Ok(_) = home.register()
          let assert Ok(_) = search.register()
          let assert Ok(_) = activity.register()
          let assert Ok(_) = imports.register()
          let assert Ok(_) = config.register()
          let assert Ok(_) = series.register()
          let assert Ok(_) = elements.register()
          Nil
        }
        Error(_) -> stg |> storage.remove_item("tankobon_account")
      }
    _ -> Nil
  }
  let assert Ok(_) = lustre.start(app, "#app", Nil)
}

fn init(_) {
  let route =
    modem.initial_uri()
    |> fn(uri) {
      case uri {
        Ok(a) -> router.uri_to_route(a)
        _ -> router.Home
      }
    }

  let assert Ok(stg) = storage.local()
  let account = case stg |> storage.get_item("tankobon_account") {
    Ok(jsonstr) ->
      json.parse(jsonstr, auth.account_decoder()) |> option.from_result
    _ -> option.None
  }

  #(
    Model(
      route:,
      account:,
      mobile_menu_open: False,
      last_refreshed_at: option.None,
      finished: case account {
        option.Some(_) -> True
        option.None -> False
      },
      status: status_api.idle(),
    ),
    case account {
      option.Some(acc) ->
        effect.batch([
          modem.init(fn(url) { router.uri_to_route(url) |> ChangeRoute }),
          auth.refresh(acc.refresh_token, RefreshAuthResponse),
          status_api.get_status(acc.access_token, StatusResponse),
          poll_status_on_interval(),
        ])
      option.None -> auth.exists(ExistsResponse)
    },
  )
}

fn poll_status_on_interval() -> effect.Effect(Msg) {
  effect.from(fn(dispatch) {
    global.set_interval(status_poll_interval_ms, fn() { dispatch(PollStatus) })
    Nil
  })
}

fn logout(m: Model) {
  let assert Ok(stg) = storage.local()
  let _ = stg |> storage.remove_item("tankobon_account")
  let assert Ok(url) = uri.parse("/login")
  #(Model(..m, account: option.None, mobile_menu_open: False), modem.load(url))
}

fn update(m: Model, msg: Msg) {
  case msg {
    ChangeRoute(route) -> {
      let now = date.now() |> date.get_time
      let should_refresh = case m.last_refreshed_at {
        option.None -> True
        option.Some(t) -> now - t > refresh_interval_ms
      }
      let refresh_effect = case should_refresh, m.account {
        True, option.Some(acc) ->
          auth.refresh(acc.refresh_token, RefreshAuthResponse)
        _, _ -> effect.none()
      }
      #(Model(..m, route:, mobile_menu_open: False), refresh_effect)
    }
    ToggleMenu -> #(
      Model(..m, mobile_menu_open: !m.mobile_menu_open),
      effect.none(),
    )
    Logout -> logout(m)
    RefreshAuth ->
      case m.account {
        option.Some(acc) -> #(
          m,
          auth.refresh(acc.refresh_token, RefreshAuthResponse),
        )
        option.None -> #(m, effect.none())
      }
    RefreshAuthResponse(Ok(account)) -> {
      let assert Ok(stg) = storage.local()
      let _ =
        stg
        |> storage.set_item(
          "tankobon_account",
          json.to_string(auth.account_to_json(account)),
        )
      #(
        Model(
          ..m,
          account: option.Some(account),
          last_refreshed_at: option.Some(date.now() |> date.get_time),
        ),
        effect.none(),
      )
    }
    RefreshAuthResponse(Error(rsvp.HttpError(resp)))
      if resp.status == 401 || resp.status == 403
    -> logout(m)
    RefreshAuthResponse(Error(_)) -> #(m, effect.none())
    ExistsResponse(Ok(exists)) ->
      case exists {
        True ->
          case m.account {
            option.Some(_) -> #(m, effect.none())
            option.None ->
              case m.route {
                router.Login -> #(Model(..m, finished: True), effect.none())
                _ -> {
                  let assert Ok(url) = uri.parse("/login")
                  #(m, modem.load(url))
                }
              }
          }
        False ->
          case m.route {
            router.Register -> #(Model(..m, finished: True), effect.none())
            _ -> {
              let assert Ok(url) = uri.parse("/register")
              #(m, modem.load(url))
            }
          }
      }
    ExistsResponse(Error(_)) -> #(m, effect.none())
    Scan ->
      case m.account {
        option.Some(acc) -> #(
          m,
          activity_api.scan(acc.access_token, ScanResponse),
        )
        option.None -> #(m, effect.none())
      }
    RefreshAllMetadata ->
      case m.account {
        option.Some(acc) -> #(
          m,
          series_api.refresh_all_metadata(
            acc.access_token,
            RefreshAllMetadataResponse,
          ),
        )
        option.None -> #(m, effect.none())
      }
    ScanResponse(_) | RefreshAllMetadataResponse(_) -> #(m, effect.none())
    PollStatus ->
      case m.account {
        option.Some(acc) -> #(
          m,
          status_api.get_status(acc.access_token, StatusResponse),
        )
        option.None -> #(m, effect.none())
      }
    StatusResponse(Ok(status)) -> #(Model(..m, status:), effect.none())
    StatusResponse(Error(_)) -> #(m, effect.none())
  }
}

fn view(m: Model) {
  case m.finished {
    False -> element.none()
    True ->
      case m.route {
        router.Login -> login.element()
        router.Register -> register.element()
        _ -> app_shell(m)
      }
  }
}

fn app_shell(m: Model) {
  html.div([attribute.class("flex-1 min-h-0 flex flex-col")], [
    html.nav(
      [
        attribute.class(
          "shrink-0 relative z-50 bg-zinc-900 border-b border-zinc-800 px-4 sm:px-6 flex items-center gap-4 h-16",
        ),
      ],
      [
        html.a(
          [
            attribute.href("/"),
            attribute.class(
              "font-[DM_Serif_Display,serif] text-pink-400 text-xl shrink-0",
            ),
          ],
          [element.text("Tankobon")],
        ),

        html.div([attribute.class("hidden sm:flex items-center gap-1")], [
          nav_item("/", "Home", "ph-house", m.route == router.Home),
          nav_item(
            "/search",
            "Search",
            "ph-magnifying-glass",
            m.route == router.Search,
          ),
          nav_item(
            "/activity",
            "Activity",
            "ph-arrow-circle-down",
            m.route == router.Activity,
          ),
          nav_item(
            "/imports",
            "Imports",
            "ph-list-checks",
            m.route == router.Imports,
          ),
          nav_item("/config", "Settings", "ph-gear", m.route == router.Config),
        ]),

        html.div([attribute.class("flex-1")], []),

        activity_indicator(m.status),

        button.icon("ph ph-list", [
          button.ghost(),
          attribute.class("sm:hidden"),
          event.on_click(ToggleMenu),
          attribute.attribute("aria-expanded", case m.mobile_menu_open {
            True -> "true"
            False -> "false"
          }),
        ]),

        html.div([attribute.class("hidden sm:flex gap-2")], [
          button.icon("ph ph-arrow-clockwise", [
            button.ghost(),
            event.on_click(RefreshAllMetadata),
            attribute.title("Refresh metadata for all series"),
          ]),
          button.icon("ph ph-magnifying-glass", [
            button.ghost(),
            event.on_click(Scan),
            attribute.title("Trigger Library Scan"),
          ]),
          button.icon("ph-fill ph-sign-out", [
            button.ghost(),
            event.on_click(Logout),
            attribute.title("Sign out"),
          ]),
        ]),

        case m.mobile_menu_open {
          True ->
            html.div(
              [
                attribute.class(
                  "sm:hidden absolute top-16 inset-x-0 bg-zinc-900 border-b border-zinc-800 p-2 flex flex-col gap-1 z-40",
                ),
              ],
              [
                nav_item("/", "Home", "ph-house", m.route == router.Home),
                nav_item(
                  "/search",
                  "Search",
                  "ph-magnifying-glass",
                  m.route == router.Search,
                ),
                nav_item(
                  "/activity",
                  "Activity",
                  "ph-arrow-circle-down",
                  m.route == router.Activity,
                ),
                nav_item(
                  "/imports",
                  "Imports",
                  "ph-list-checks",
                  m.route == router.Imports,
                ),
                nav_item(
                  "/config",
                  "Config",
                  "ph-gear",
                  m.route == router.Config,
                ),
                button.icon_label("ph-fill ph-sign-out", "Sign out", [
                  button.ghost(),
                  attribute.class(
                    "text-zinc-500 hover:text-red-400 hover:bg-red-400/10",
                  ),
                  event.on_click(Logout),
                ]),
              ],
            )
          False -> element.none()
        },
      ],
    ),
    html.div([attribute.class("flex-1 min-h-0 overflow-auto")], [
      case m.route {
        router.Home -> home.element()
        router.Search -> search.element()
        router.Activity -> activity.element()
        router.Imports -> imports.element()
        router.Config -> config.element()
        router.Series(id) -> series.element(id)
        router.Elements -> elements.element()
        _ -> element.none()
      },
    ]),
  ])
}

fn activity_indicator(status: status_api.Status) {
  let text = case status.worker, status.searcher {
    "", "" -> option.None
    worker, "" -> option.Some(worker)
    "", searcher -> option.Some(searcher)
    worker, searcher -> option.Some(worker <> " · " <> searcher)
  }

  case text {
    option.None -> element.none()
    option.Some(text) ->
      html.div(
        [
          attribute.class(
            "hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-zinc-800/60 text-xs text-zinc-400 max-w-xs",
          ),
          attribute.title(text),
        ],
        [
          html.span([attribute.class("relative flex h-2 w-2 shrink-0")], [
            html.span(
              [
                attribute.class(
                  "animate-ping absolute inline-flex h-full w-full rounded-full bg-pink-400 opacity-75",
                ),
              ],
              [],
            ),
            html.span(
              [
                attribute.class(
                  "relative inline-flex rounded-full h-2 w-2 bg-pink-500",
                ),
              ],
              [],
            ),
          ]),
          html.span([attribute.class("truncate")], [element.text(text)]),
        ],
      )
  }
}

fn nav_item(href: String, name: String, icon: String, active: Bool) {
  html.a(
    [
      attribute.href(href),
      attribute.class(
        "flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors shrink-0 outline-none",
      ),
      case active {
        True -> attribute.class("text-pink-400 bg-zinc-400/10")
        False -> attribute.class("text-zinc-500 hover:text-zinc-200")
      },
    ],
    [
      html.i([attribute.class("text-lg ph-fill " <> icon)], []),
      html.span([attribute.class("inline")], [element.text(name)]),
    ],
  )
}
