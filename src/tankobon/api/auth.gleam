import gleam/dynamic/decode
import gleam/http
import gleam/http/request
import gleam/json
import rsvp
import tankobon/api/api

pub type Account {
  Account(username: String, access_token: String, refresh_token: String)
}

pub fn account_decoder() -> decode.Decoder(Account) {
  use username <- decode.field("username", decode.string)
  use refresh_token <- decode.field("refreshToken", decode.string)
  use access_token <- decode.field("accessToken", decode.string)
  decode.success(Account(username:, access_token:, refresh_token:))
}

pub fn account_to_json(account: Account) -> json.Json {
  json.object([
    #("username", json.string(account.username)),
    #("accessToken", json.string(account.access_token)),
    #("refreshToken", json.string(account.refresh_token)),
  ])
}

pub fn login(username: String, password: String, resp: api.Response(Account, a)) {
  let req =
    json.object([
      #("username", json.string(username)),
      #("password", json.string(password)),
    ])

  rsvp.post(
    api.create_url("/api/login"),
    req,
    rsvp.expect_json(account_decoder(), resp),
  )
}

pub fn refresh(refresh_token: String, resp: api.Response(Account, a)) {
  let assert Ok(req) = request.to(api.create_url("/api/refresh"))
  let req =
    req
    |> request.set_method(http.Post)
    |> request.set_header("Authorization", "Bearer " <> refresh_token)
  rsvp.send(req, rsvp.expect_json(account_decoder(), resp))
}

pub fn register(
  username: String,
  password: String,
  resp: api.Response(Account, a),
) {
  let req =
    json.object([
      #("username", json.string(username)),
      #("password", json.string(password)),
    ])

  rsvp.post(
    api.create_url("/api/register"),
    req,
    rsvp.expect_json(account_decoder(), resp),
  )
}

pub fn exists(resp: api.Response(Bool, a)) {
  rsvp.get(api.create_url("/api/exists"), rsvp.expect_json(decode.bool, resp))
}
