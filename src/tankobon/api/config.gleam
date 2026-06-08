import gleam/dynamic/decode
import gleam/http
import gleam/http/request
import gleam/json
import rsvp
import tankobon/api/api

pub type Config {
  Config(
    prowlarr_url: String,
    prowlarr_api_token: String,
    qbittorrent_url: String,
    qbittorrent_user: String,
    qbittorrent_pass: String,
    qbittorrent_category: String,
    kavita_url: String,
    kavita_api_key: String,
    library_path: String,
  )
}

pub fn config_decoder() -> decode.Decoder(Config) {
  use prowlarr_url <- decode.field("prowlarrURL", decode.string)
  use prowlarr_api_token <- decode.field("prowlarrAPIToken", decode.string)
  use qbittorrent_url <- decode.field("qbittorrentURL", decode.string)
  use qbittorrent_user <- decode.field("qbittorrentUser", decode.string)
  use qbittorrent_pass <- decode.field("qbittorrentPass", decode.string)
  use qbittorrent_category <- decode.field("qbittorrentCategory", decode.string)
  use kavita_url <- decode.field("kavitaURL", decode.string)
  use kavita_api_key <- decode.field("kavitaAPIKey", decode.string)
  use library_path <- decode.field("libraryPath", decode.string)
  decode.success(Config(
    prowlarr_url:,
    prowlarr_api_token:,
    qbittorrent_url:,
    qbittorrent_user:,
    qbittorrent_pass:,
    qbittorrent_category:,
    kavita_url:,
    kavita_api_key:,
    library_path:,
  ))
}

pub fn get_config(token: String, resp: api.Response(Config, a)) {
  let assert Ok(req) = request.to(api.create_url("/api/config"))
  let req =
    req
    |> request.set_method(http.Get)
    |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(req, rsvp.expect_json(config_decoder(), resp))
}

pub fn save_config(cfg: Config, token: String, resp: api.Response(Nil, a)) {
  let body =
    json.object([
      #("prowlarrURL", json.string(cfg.prowlarr_url)),
      #("prowlarrAPIToken", json.string(cfg.prowlarr_api_token)),
      #("qbittorrentURL", json.string(cfg.qbittorrent_url)),
      #("qbittorrentUser", json.string(cfg.qbittorrent_user)),
      #("qbittorrentPass", json.string(cfg.qbittorrent_pass)),
      #("qbittorrentCategory", json.string(cfg.qbittorrent_category)),
      #("libraryPath", json.string(cfg.library_path)),
    ])

  let assert Ok(req) = request.to(api.create_url("/api/config"))
  let req =
    req
    |> request.set_method(http.Post)
    |> request.set_body(body |> json.to_string)
    |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(req, api.expect_ok_response(resp))
}
