import gleam/dynamic/decode
import gleam/http
import gleam/http/request
import gleam/json
import gleam/option
import rsvp
import tankobon/api/api
import tankobon/api/series as series_api

pub type PendingReason {
  Unmatched
  NoFiles
  NeedsReview
  Unknown(reason: String)
}

pub type TorrentInfo {
  TorrentInfo(hash: String, name: String, state: String, progress: Float)
}

pub type PendingImport {
  PendingImport(
    torrent: TorrentInfo,
    reason: PendingReason,
    series: option.Option(series_api.Series),
    shape: String,
  )
}

pub type FileMapping {
  FileMapping(
    path: String,
    volumes: List(String),
    chapters: List(String),
    special: Bool,
  )
}

pub fn reason_from_string(s: String) -> PendingReason {
  case s {
    "unmatched" -> Unmatched
    "no_files" -> NoFiles
    "needs_review" -> NeedsReview
    _ -> Unknown(reason: s)
  }
}

pub fn torrent_info_decoder() -> decode.Decoder(TorrentInfo) {
  use hash <- decode.field("hash", decode.string)
  use name <- decode.field("name", decode.string)
  use state <- decode.field("state", decode.string)
  use progress <- decode.field("progress", decode.float)
  decode.success(TorrentInfo(hash:, name:, state:, progress:))
}

pub fn pending_import_decoder() -> decode.Decoder(PendingImport) {
  use torrent <- decode.field("torrent", torrent_info_decoder())
  use reason <- decode.field("reason", decode.string)
  use series <- decode.optional_field(
    "series",
    option.None,
    decode.map(series_api.series_decoder(), option.Some),
  )
  use shape <- decode.optional_field("shape", "", decode.string)
  decode.success(PendingImport(
    torrent:,
    reason: reason_from_string(reason),
    series:,
    shape:,
  ))
}

pub fn file_mapping_decoder() -> decode.Decoder(FileMapping) {
  use path <- decode.field("path", decode.string)
  use volumes <- decode.field("volumes", decode.list(decode.string))
  use chapters <- decode.field("chapters", decode.list(decode.string))
  use special <- decode.field("special", decode.bool)
  decode.success(FileMapping(path:, volumes:, chapters:, special:))
}

pub fn list_imports(token: String, resp: api.Response(List(PendingImport), a)) {
  let assert Ok(req) = request.to(api.create_url("/api/imports"))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(
    req,
    rsvp.expect_json(
      {
        use imports <- decode.field(
          "imports",
          decode.list(pending_import_decoder()),
        )
        decode.success(imports)
      },
      resp,
    ),
  )
}

pub fn get_import_files(
  hash: String,
  token: String,
  resp: api.Response(List(FileMapping), a),
) {
  let assert Ok(req) =
    request.to(api.create_url("/api/imports/" <> hash <> "/files"))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(
    req,
    rsvp.expect_json(
      {
        use files <- decode.field("files", decode.list(file_mapping_decoder()))
        decode.success(files)
      },
      resp,
    ),
  )
}

pub type ImportLog {
  ImportLog(
    id: Int,
    created_at: String,
    series_title: String,
    torrent_name: String,
    volumes: List(String),
    chapters: List(String),
  )
}

pub fn import_log_decoder() -> decode.Decoder(ImportLog) {
  use id <- decode.field("id", decode.int)
  use created_at <- decode.field("createdAt", decode.string)
  use series_title <- decode.field("seriesTitle", decode.string)
  use torrent_name <- decode.field("torrentName", decode.string)
  use content <- decode.field("content", {
    use volumes <- decode.field("volumes", decode.list(decode.string))
    use chapters <- decode.field("chapters", decode.list(decode.string))
    decode.success(#(volumes, chapters))
  })
  decode.success(ImportLog(
    id:,
    created_at:,
    series_title:,
    torrent_name:,
    volumes: content.0,
    chapters: content.1,
  ))
}

pub fn get_history(token: String, resp: api.Response(List(ImportLog), a)) {
  let assert Ok(req) = request.to(api.create_url("/api/imports/history"))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(
    req,
    rsvp.expect_json(
      {
        use history <- decode.field(
          "history",
          decode.list(import_log_decoder()),
        )
        decode.success(history)
      },
      resp,
    ),
  )
}

pub fn manual_import(
  hash: String,
  series_id: Int,
  file_mappings: List(FileMapping),
  token: String,
  resp: api.Response(Nil, a),
) {
  let assert Ok(req) = request.to(api.create_url("/api/imports/" <> hash))
  let req =
    req
    |> request.set_method(http.Post)
    |> request.set_body(
      json.object([
        #("seriesId", json.int(series_id)),
        #(
          "fileMappings",
          json.array(file_mappings, fn(m) {
            json.object([
              #("path", json.string(m.path)),
              #("volumes", json.array(m.volumes, json.string)),
              #("chapters", json.array(m.chapters, json.string)),
              #("special", json.bool(m.special)),
            ])
          }),
        ),
      ])
      |> json.to_string,
    )
    |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(req, api.expect_ok_response(resp))
}
