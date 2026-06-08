import formal/form
import gleam/json
import lustre
import lustre/attribute
import lustre/effect
import lustre/element
import lustre/element/html
import lustre/event
import plinth/javascript/storage
import rsvp
import tankobon/api/auth
import tankobon/api/config as config_api
import tankobon/ui/button
import tankobon/ui/display
import tankobon/ui/input

pub type Model {
  Model(
    account: auth.Account,
    form: form.Form(config_api.Config),
    loading: Bool,
    saved: Bool,
  )
}

pub type Msg {
  LoadConfig
  LoadConfigResponse(Result(config_api.Config, rsvp.Error(String)))
  FormSubmitted(Result(config_api.Config, form.Form(config_api.Config)))
  SaveConfigResponse(Result(Nil, rsvp.Error(String)))
}

pub fn register() {
  let app = lustre.component(init, update, view, [])
  lustre.register(app, "config-page")
}

pub fn element() {
  element.element("config-page", [], [])
}

fn config_form(cfg: config_api.Config) {
  form.new({
    use prowlarr_url <- form.field(
      "prowlarr_url",
      form.parse_string |> form.check_not_empty,
    )
    use prowlarr_api_token <- form.field(
      "prowlarr_api_token",
      form.parse_string |> form.check_not_empty,
    )
    use qbittorrent_url <- form.field(
      "qbittorrent_url",
      form.parse_string |> form.check_not_empty,
    )
    use qbittorrent_user <- form.field(
      "qbittorrent_user",
      form.parse_string |> form.check_not_empty,
    )
    use qbittorrent_pass <- form.field(
      "qbittorrent_pass",
      form.parse_string |> form.check_not_empty,
    )
    use qbittorrent_category <- form.field(
      "qbittorrent_category",
      form.parse_string,
    )
    use kavita_url <- form.field("kavita_url", form.parse_string)
    use kavita_api_key <- form.field("kavita_api_key", form.parse_string)
    use library_path <- form.field(
      "library_path",
      form.parse_string |> form.check_not_empty,
    )
    form.success(config_api.Config(
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
  })
  |> form.add_values([
    #("prowlarr_url", cfg.prowlarr_url),
    #("prowlarr_api_token", cfg.prowlarr_api_token),
    #("qbittorrent_url", cfg.qbittorrent_url),
    #("qbittorrent_user", cfg.qbittorrent_user),
    #("qbittorrent_pass", cfg.qbittorrent_pass),
    #("qbittorrent_category", cfg.qbittorrent_category),
    #("kavita_url", cfg.kavita_url),
    #("kavita_api_key", cfg.kavita_api_key),
    #("library_path", cfg.library_path),
  ])
}

fn init(_) {
  let assert Ok(stg) = storage.local()
  let assert Ok(account_json) = stg |> storage.get_item("tankobon_account")
  let assert Ok(account) = json.parse(account_json, auth.account_decoder())
  #(
    Model(
      account:,
      form: config_form(config_api.Config("", "", "", "", "", "", "", "", "")),
      loading: True,
      saved: False,
    ),
    config_api.get_config(account.access_token, LoadConfigResponse),
  )
}

fn update(m: Model, msg: Msg) {
  case msg {
    LoadConfig -> #(
      Model(..m, loading: True),
      config_api.get_config(m.account.access_token, LoadConfigResponse),
    )
    LoadConfigResponse(Ok(cfg)) -> #(
      Model(..m, form: config_form(cfg), loading: False),
      effect.none(),
    )
    LoadConfigResponse(Error(_)) -> #(Model(..m, loading: False), effect.none())
    FormSubmitted(Ok(cfg)) -> #(
      Model(..m, loading: True, saved: False),
      config_api.save_config(cfg, m.account.access_token, SaveConfigResponse),
    )
    FormSubmitted(Error(f)) -> #(Model(..m, form: f), effect.none())
    SaveConfigResponse(Ok(_)) -> #(
      Model(..m, loading: False, saved: True),
      config_api.get_config(m.account.access_token, LoadConfigResponse),
    )
    SaveConfigResponse(Error(_)) -> #(Model(..m, loading: False), effect.none())
  }
}

fn view(m: Model) {
  let submit = fn(fields) {
    config_form(config_api.Config("", "", "", "", "", "", "", "", ""))
    |> form.add_values(fields)
    |> form.run
    |> FormSubmitted
  }

  html.div(
    [
      attribute.class(
        "min-h-full bg-zinc-950 text-white flex justify-center px-4 sm:px-8 py-4 sm:py-8",
      ),
    ],
    [
      html.div([attribute.class("w-full max-w-2xl flex flex-col gap-8")], [
        case m.loading {
          True -> display.loading()
          False ->
            html.form(
              [
                attribute.class("flex flex-col gap-8"),
                event.on_submit(submit),
              ],
              [
                config_section("Prowlarr", [
                  input.labeled_input(
                    "URL",
                    "prowlarr_url",
                    form.field_error_messages(m.form, "prowlarr_url"),
                    [
                      attribute.placeholder("http://localhost:9696"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(m.form, "prowlarr_url")),
                    ],
                  ),
                  input.labeled_input(
                    "API Token",
                    "prowlarr_api_token",
                    form.field_error_messages(m.form, "prowlarr_api_token"),
                    [
                      attribute.type_("password"),
                      attribute.placeholder("Your Prowlarr API token"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(
                        m.form,
                        "prowlarr_api_token",
                      )),
                    ],
                  ),
                ]),
                config_section("qBittorrent", [
                  input.labeled_input(
                    "URL",
                    "qbittorrent_url",
                    form.field_error_messages(m.form, "qbittorrent_url"),
                    [
                      attribute.placeholder("http://localhost:8080"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(
                        m.form,
                        "qbittorrent_url",
                      )),
                    ],
                  ),
                  input.labeled_input(
                    "Username",
                    "qbittorrent_user",
                    form.field_error_messages(m.form, "qbittorrent_user"),
                    [
                      attribute.placeholder("admin"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(
                        m.form,
                        "qbittorrent_user",
                      )),
                    ],
                  ),
                  input.labeled_input(
                    "Password",
                    "qbittorrent_pass",
                    form.field_error_messages(m.form, "qbittorrent_pass"),
                    [
                      attribute.type_("password"),
                      attribute.placeholder("••••••••"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(
                        m.form,
                        "qbittorrent_pass",
                      )),
                    ],
                  ),
                  input.labeled_input(
                    "Category",
                    "qbittorrent_category",
                    form.field_error_messages(m.form, "qbittorrent_category"),
                    [
                      attribute.placeholder(
                        "(optional) torrent category to use",
                      ),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(
                        m.form,
                        "qbittorrent_category",
                      )),
                    ],
                  ),
                ]),
                config_section("Kavita", [
                  input.labeled_input(
                    "URL",
                    "kavita_url",
                    form.field_error_messages(m.form, "kavita_url"),
                    [
                      attribute.placeholder("http://localhost:8080"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(m.form, "kavita_url")),
                    ],
                  ),
                  input.labeled_input(
                    "API Key",
                    "kavita_api_key",
                    form.field_error_messages(m.form, "kavita_api_key"),
                    [
                      attribute.type_("password"),
                      attribute.placeholder("Kavita API key"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(m.form, "kavita_api_key")),
                    ],
                  ),
                ]),
                config_section("Library", [
                  input.labeled_input(
                    "Library Path",
                    "library_path",
                    form.field_error_messages(m.form, "library_path"),
                    [
                      attribute.placeholder("/mnt/library"),
                      attribute.disabled(m.loading),
                      attribute.value(form.field_value(m.form, "library_path")),
                    ],
                  ),
                ]),
                button.button("Save Settings", [
                  button.primary(),
                  attribute.type_("submit"),
                  attribute.class("w-full disabled:bg-zinc-700"),
                  attribute.disabled(m.loading),
                ]),
              ],
            )
        },
      ]),
    ],
  )
}

fn config_section(
  title: String,
  fields: List(element.Element(Msg)),
) -> element.Element(Msg) {
  html.div([attribute.class("flex flex-col gap-4")], [
    display.section_heading(title, []),
    html.div(
      [attribute.class("bg-zinc-900 rounded-lg p-4 flex flex-col gap-4")],
      fields,
    ),
  ])
}
