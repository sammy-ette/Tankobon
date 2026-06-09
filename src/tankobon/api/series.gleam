import gleam/dynamic/decode
import gleam/http
import gleam/http/request
import gleam/int
import gleam/json
import gleam/string
import rsvp
import tankobon/api/api

pub type Series {
  Series(
    id: Int,
    title: String,
    alt_titles: List(String),
    cover_url: String,
    status: String,
    year: Int,
    overview: String,
    total_volumes: Int,
    total_chapters: Int,
    last_checked_at: String,
    monitored: Bool,
    monitor_chapters: Bool,
    imported_volumes: List(String),
    imported_chapters: List(String),
    unmonitored_volumes: List(String),
  )
}

pub fn series_decoder() -> decode.Decoder(Series) {
  use id <- decode.field("id", decode.int)
  use title <- decode.field("title", decode.string)
  use alt_titles <- decode.field("altTitles", decode.list(decode.string))
  use cover_url <- decode.field("coverUrl", decode.string)
  use status <- decode.field("status", decode.string)
  use year <- decode.field("year", decode.int)
  use overview <- decode.field("overview", decode.string)
  use total_volumes <- decode.field("totalVolumes", decode.int)
  use total_chapters <- decode.field("totalChapters", decode.int)
  use last_checked_at <- decode.field(
    "lastCheckedAt",
    decode.one_of(decode.string, [decode.success("")]),
  )
  use monitored <- decode.field("monitored", decode.bool)
  use monitor_chapters <- decode.optional_field(
    "monitorChapters",
    True,
    decode.bool,
  )
  use imported_volumes <- decode.optional_field(
    "imported",
    [],
    decode.at(["volumes"], decode.list(decode.string)),
  )
  use imported_chapters <- decode.optional_field(
    "imported",
    [],
    decode.at(["chapters"], decode.list(decode.string)),
  )
  use unmonitored_volumes <- decode.optional_field(
    "unmonitoredVolumes",
    [],
    decode.list(decode.string),
  )
  decode.success(Series(
    id:,
    title:,
    alt_titles:,
    cover_url:,
    status: status |> string.capitalise,
    year:,
    overview:,
    total_volumes:,
    total_chapters:,
    last_checked_at:,
    monitored:,
    monitor_chapters:,
    imported_volumes:,
    imported_chapters:,
    unmonitored_volumes:,
  ))
}

pub type SearchResult {
  SearchResult(
    id: Int,
    title: String,
    alt_titles: List(String),
    cover_url: String,
    status: String,
    year: Int,
    overview: String,
    total_volumes: Int,
  )
}

pub fn search_result_decoder() -> decode.Decoder(SearchResult) {
  use id <- decode.field("id", decode.int)
  use title <- decode.field("title", decode.string)
  use alt_titles <- decode.field("altTitles", decode.list(decode.string))
  use cover_url <- decode.field("coverUrl", decode.string)
  use status <- decode.field("status", decode.string)
  use year <- decode.field("year", decode.int)
  use overview <- decode.field("overview", decode.string)
  use total_volumes <- decode.field("totalVolumes", decode.int)
  decode.success(SearchResult(
    id:,
    title:,
    alt_titles:,
    cover_url:,
    status: status |> string.capitalise,
    year:,
    overview:,
    total_volumes:,
  ))
}

pub type Candidate {
  Candidate(
    title: String,
    indexer: String,
    seeders: Int,
    leechers: Int,
    size: Int,
    download_url: String,
    score: Float,
    shape: String,
    approved: Bool,
    reject_reason: String,
    grabbed: Bool,
  )
}

pub fn candidate_decoder() -> decode.Decoder(Candidate) {
  use title <- decode.subfield(["result", "title"], decode.string)
  use indexer <- decode.subfield(["result", "indexer"], decode.string)
  use seeders <- decode.subfield(["result", "seeders"], decode.int)
  use leechers <- decode.subfield(["result", "leechers"], decode.int)
  use size <- decode.subfield(["result", "size"], decode.int)
  use download_url <- decode.subfield(
    ["result", "downloadUrl"],
    decode.string,
  )
  use score <- decode.field("score", decode.float)
  use shape <- decode.optional_field("shape", "", decode.string)
  use approved <- decode.field("approved", decode.bool)
  use reject_reason <- decode.optional_field("rejectReason", "", decode.string)
  use grabbed <- decode.optional_field("grabbed", False, decode.bool)
  decode.success(Candidate(
    title:,
    indexer:,
    seeders:,
    leechers:,
    size:,
    download_url:,
    score:,
    shape:,
    approved:,
    reject_reason:,
    grabbed:,
  ))
}

pub type ReleaseSearch {
  ReleaseSearch(
    status: String,
    candidates: List(Candidate),
    error: String,
  )
}

pub fn release_search_decoder() -> decode.Decoder(ReleaseSearch) {
  use status <- decode.field("status", decode.string)
  use candidates <- decode.optional_field(
    "candidates",
    [],
    decode.list(candidate_decoder()),
  )
  use error <- decode.optional_field("error", "", decode.string)
  decode.success(ReleaseSearch(status:, candidates:, error:))
}

pub fn find_releases(
  id: Int,
  token: String,
  resp: api.Response(ReleaseSearch, a),
) {
  let assert Ok(req) =
    request.to(
      api.create_url("/api/series/" <> int.to_string(id) <> "/search/releases"),
    )
  let req =
    req
    |> request.set_method(http.Post)
    |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(req, rsvp.expect_json(release_search_decoder(), resp))
}

pub fn get_release_search(
  id: Int,
  token: String,
  resp: api.Response(ReleaseSearch, a),
) {
  let assert Ok(req) =
    request.to(
      api.create_url("/api/series/" <> int.to_string(id) <> "/search/releases"),
    )
  let req = req |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(req, rsvp.expect_json(release_search_decoder(), resp))
}

pub fn grab(
  id: Int,
  download_url: String,
  title: String,
  token: String,
  resp: api.Response(Nil, a),
) {
  let body =
    json.object([
      #("downloadUrl", json.string(download_url)),
      #("title", json.string(title)),
    ])

  let assert Ok(req) =
    request.to(api.create_url("/api/series/" <> int.to_string(id) <> "/grab"))
  let req =
    req
    |> request.set_method(http.Post)
    |> request.set_body(body |> json.to_string)
    |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(req, api.expect_ok_response(resp))
}

pub fn list(token: String, resp: api.Response(List(Series), a)) {
  let assert Ok(req) = request.to(api.create_url("/api/series"))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(
    req,
    rsvp.expect_json(
      {
        use series <- decode.field("series", decode.list(series_decoder()))
        decode.success(series)
      },
      resp,
    ),
  )
}

pub fn search(
  query: String,
  token: String,
  resp: api.Response(List(SearchResult), a),
) {
  let assert Ok(req) =
    request.to(api.create_url_with_query("/api/series/search", [#("q", query)]))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(
    req,
    rsvp.expect_json(
      {
        use results <- decode.field(
          "results",
          decode.list(search_result_decoder()),
        )
        decode.success(results)
      },
      resp,
    ),
  )
}

pub fn add(manga_baka_id: Int, token: String, resp: api.Response(Int, a)) {
  let body =
    json.object([
      #("mangaBakaId", json.int(manga_baka_id)),
    ])

  let assert Ok(req) = request.to(api.create_url("/api/series"))
  let req =
    req
    |> request.set_method(http.Post)
    |> request.set_body(body |> json.to_string)
    |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(
    req,
    rsvp.expect_ok_response(fn(res) {
      case res {
        Ok(_) -> resp(Ok(manga_baka_id))
        Error(e) -> resp(Error(e))
      }
    }),
  )
}

pub fn get(id: Int, token: String, resp: api.Response(Series, a)) {
  let assert Ok(req) =
    request.to(api.create_url("/api/series/" <> int.to_string(id)))
  let req = req |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(req, rsvp.expect_json(series_decoder(), resp))
}

pub fn patch(
  id: Int,
  body: json.Json,
  token: String,
  resp: api.Response(Series, a),
) {
  let assert Ok(req) =
    request.to(api.create_url("/api/series/" <> int.to_string(id)))
  let req =
    req
    |> request.set_method(http.Patch)
    |> request.set_body(body |> json.to_string)
    |> request.set_header("Authorization", "Bearer " <> token)
  rsvp.send(req, rsvp.expect_json(series_decoder(), resp))
}

pub fn delete(
  id: Int,
  token: String,
  delete_files: Bool,
  resp: api.Response(Nil, a),
) {
  let assert Ok(req) =
    request.to(
      api.create_url_with_query(
        "/api/series/" <> int.to_string(id),
        case delete_files {
          True -> [#("deleteFiles", "true")]
          False -> []
        },
      ),
    )

  let req =
    req
    |> request.set_method(http.Delete)
    |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(req, api.expect_ok_response(resp))
}

pub fn search_on_series(id: Int, token: String, resp: api.Response(Nil, a)) {
  let assert Ok(req) =
    request.to(api.create_url("/api/series/" <> int.to_string(id) <> "/search"))
  let req =
    req
    // |> request.set_method(http.Post)
    |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(req, api.expect_ok_response(resp))
}

pub fn refresh_metadata(id: Int, token: String, resp: api.Response(Series, a)) {
  let assert Ok(req) =
    request.to(api.create_url("/api/series/" <> int.to_string(id) <> "/refresh"))
  let req =
    req
    // |> request.set_method(http.Post)
    |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(req, rsvp.expect_json(series_decoder(), resp))
}

pub fn refresh_all_metadata(token: String, resp: api.Response(Nil, a)) {
  let assert Ok(req) = request.to(api.create_url("/api/series/refresh"))
  let req =
    req
    // |> request.set_method(http.Post)
    |> request.set_header("Authorization", "Bearer " <> token)

  rsvp.send(req, api.expect_ok_response(resp))
}
