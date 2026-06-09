import gleam/dynamic/decode
import gleam/http/request
import rsvp
import tankobon/api/api

pub type Status {
  Status(worker: String, searcher: String)
}

fn status_decoder() -> decode.Decoder(Status) {
  use worker <- decode.optional_field("worker", "", decode.string)
  use searcher <- decode.optional_field("searcher", "", decode.string)
  decode.success(Status(worker:, searcher:))
}

pub fn idle() -> Status {
  Status(worker: "", searcher: "")
}

pub fn get_status(token: String, resp: api.Response(Status, a)) {
  let assert Ok(req) = request.to(api.create_url("/api/status"))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(req, rsvp.expect_json(status_decoder(), resp))
}
