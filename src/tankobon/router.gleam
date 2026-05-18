import gleam/int
import gleam/option
import gleam/string
import gleam/uri
import plinth/browser/location

import plinth/browser/window

pub type Msg {
  ChangeRoute(Route)
}

pub type Route {
  Home
  Login
  Register
  Search
  Activity
  Imports
  Config
  Elements
  Series(Int)
  Unknown
}

pub fn uri_to_route(uri: uri.Uri) -> Route {
  let _params = case uri.query {
    option.Some(q) ->
      case uri.parse_query(q) {
        Ok(p) -> p
        Error(_) -> []
      }
    option.None -> []
  }

  let router = fn(path: String) {
    case path {
      "/" | "" -> Home
      "/login" -> Login
      "/register" -> Register
      "/search" -> Search
      "/activity" -> Activity
      "/imports" -> Imports
      "/config" -> Config
      "/elements" -> Elements
      "/series/" <> id -> {
        case int.parse(id) {
          Ok(i) -> Series(i)
          Error(_) -> Unknown
        }
      }
      _ -> Unknown
    }
  }

  router(uri.path)
}

pub fn direct(root: uri.Uri, rel: String) -> String {
  let assert Ok(rel_url) =
    uri.parse({ root.path <> rel } |> string.replace("//", "/"))
  let assert Ok(direction) = uri.merge(root, rel_url)
  uri.to_string(direction)
}

pub fn get_route() -> uri.Uri {
  let assert Ok(route) =
    uri.parse(window.location(window.self()) |> location.origin())
  route
}
