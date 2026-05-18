import gleam/dynamic/decode
import gleam/int
import gleam/json
import gleam/option
import gleam/string
import gleam/uri
import plinth/browser/location
import plinth/browser/window
import rsvp
import tankobon/router

pub type Response(a, b) =
  fn(Result(a, rsvp.Error(String))) -> b

pub type ApiError {
  NetworkUnavailable
  ResponseError(status: Int, message: String)
  UnexpectedError
}

pub fn handle_error(err: rsvp.Error(String)) -> ApiError {
  case err {
    rsvp.NetworkError -> NetworkUnavailable
    rsvp.HttpError(resp) -> {
      let msg = case
        json.parse(resp.body, decode.at(["error"], decode.string))
      {
        Ok(m) -> m
        Error(_) ->
          "An error occurred (status " <> int.to_string(resp.status) <> ")"
      }
      ResponseError(resp.status, msg)
    }
    _ -> UnexpectedError
  }
}

pub fn error_message(err: ApiError) -> String {
  case err {
    NetworkUnavailable ->
      "Network unavailable. Check your connection and try again."
    UnexpectedError -> "An unexpected error occurred."
    ResponseError(401, _) -> "Your session has expired. Please log in again."
    ResponseError(403, _) -> "You don't have permission to perform this action."
    ResponseError(status, _) if status >= 500 ->
      "Something went wrong. Please try again."
    ResponseError(_, msg) -> normalize(msg)
  }
}

fn normalize(msg: String) -> String {
  let msg = string.capitalise(msg)
  case
    string.ends_with(msg, ".")
    || string.ends_with(msg, "!")
    || string.ends_with(msg, "?")
  {
    True -> msg
    False -> msg <> "."
  }
}

pub fn create_url(path: String) -> String {
  let root = window.self() |> window.location() |> location.origin
  let assert Ok(root_uri) = uri.parse(root)
  router.direct(root_uri, path)
}

pub fn create_url_with_query(
  path: String,
  query: List(#(String, String)),
) -> String {
  let assert Ok(root) =
    window.self() |> window.location() |> location.origin |> uri.parse
  let assert Ok(original_uri) = uri.parse(router.direct(root, path))
  uri.Uri(..original_uri, query: option.Some(query |> uri.query_to_string))
  |> uri.to_string
}

pub fn expect_ok_response(handler: fn(Result(Nil, rsvp.Error(String))) -> a) {
  rsvp.expect_ok_response(fn(res) {
    case res {
      Ok(_) -> handler(Ok(Nil))
      Error(e) -> handler(Error(e))
    }
  })
}
