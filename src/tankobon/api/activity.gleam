import gleam/dynamic/decode
import gleam/http/request
import gleam/option
import rsvp
import tankobon/api/api
import tankobon/api/jobs as imports_api
import tankobon/api/series as series_api

pub type ActivityItem {
  ActivityItem(
    torrent: imports_api.TorrentInfo,
    series: option.Option(series_api.Series),
    processing: Bool,
  )
}

fn activity_item_decoder() -> decode.Decoder(ActivityItem) {
  use torrent <- decode.field("torrent", imports_api.torrent_info_decoder())
  use series <- decode.optional_field(
    "series",
    option.None,
    decode.map(series_api.series_decoder(), option.Some),
  )
  use processing <- decode.optional_field("processing", False, decode.bool)
  decode.success(ActivityItem(torrent:, series:, processing:))
}

pub fn get_activity(token: String, resp: api.Response(List(ActivityItem), a)) {
  let assert Ok(req) = request.to(api.create_url("/api/activity"))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(
    req,
    rsvp.expect_json(
      {
        use items <- decode.field(
          "activity",
          decode.list(activity_item_decoder()),
        )
        decode.success(items)
      },
      resp,
    ),
  )
}

pub fn scan(token: String, resp: api.Response(Nil, a)) {
  let assert Ok(req) = request.to(api.create_url("/api/scan"))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(req, api.expect_ok_response(resp))
}
