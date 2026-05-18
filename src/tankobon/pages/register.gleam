import formal/form
import gleam/json
import gleam/uri
import lustre
import lustre/attribute
import lustre/effect
import lustre/element
import lustre/element/html
import lustre/event
import modem
import plinth/javascript/storage
import rsvp
import tankobon/api/auth
import tankobon/ui/button
import tankobon/ui/input

pub type Register {
  Register(username: String, password: String, confirm_password: String)
}

pub type Model {
  Model(form: form.Form(Register), loading: Bool, error: Bool)
}

pub type Msg {
  FormSubmitted(Result(Register, form.Form(Register)))
  RegisterResponse(Result(auth.Account, rsvp.Error(String)))
}

pub fn register() {
  let app = lustre.component(init, update, view, [])
  lustre.register(app, "register-page")
}

pub fn element() {
  element.element("register-page", [attribute.class("w-screen h-screen")], [])
}

fn register_form() {
  form.new({
    use username <- form.field(
      "username",
      form.parse_string |> form.check_not_empty,
    )
    use password <- form.field(
      "password",
      form.parse_string
        |> form.check_not_empty
        |> form.check_string_length_more_than(5),
    )
    use confirm_password <- form.field(
      "confirm_password",
      form.parse_string
        |> form.check_not_empty
        |> form.check_confirms(password),
    )
    form.success(Register(username:, password:, confirm_password:))
  })
}

fn init(_) {
  #(Model(form: register_form(), loading: False, error: False), effect.none())
}

fn update(m: Model, msg: Msg) {
  case msg {
    FormSubmitted(Ok(act)) -> {
      #(
        Model(..m, loading: True, error: False),
        auth.register(act.username, act.password, RegisterResponse),
      )
    }
    FormSubmitted(Error(f)) -> {
      #(Model(..m, form: f), effect.none())
    }
    RegisterResponse(Ok(act)) -> {
      let assert Ok(stg) = storage.local()
      let _ =
        stg
        |> storage.set_item(
          "tankobon_account",
          auth.account_to_json(act) |> json.to_string,
        )
      #(
        m,
        modem.load({
          let assert Ok(url) = uri.parse("/")
          url
        }),
      )
    }
    RegisterResponse(Error(_)) -> {
      #(Model(..m, loading: False, error: True), effect.none())
    }
  }
}

fn view(m: Model) {
  let submit = fn(fields) {
    register_form()
    |> form.add_values(fields)
    |> form.run
    |> FormSubmitted
  }
  view_full_bleed(m, submit)
}

fn error_banner() {
  html.div(
    [
      attribute.class(
        "bg-red-500/10 border border-red-500/20 text-red-400 text-sm px-4 py-3 rounded-lg",
      ),
    ],
    [element.text("Registration failed. Please try again.")],
  )
}

fn form_fields(m: Model, submit) {
  html.form([event.on_submit(submit), attribute.class("flex flex-col gap-5")], [
    html.div([attribute.class("flex flex-col gap-2")], [
      input.labeled_input(
        "Username",
        "username",
        form.field_error_messages(m.form, "username"),
        [
          attribute.type_("text"),
          attribute.placeholder("admin"),
          attribute.autocomplete("username"),
          attribute.disabled(m.loading),
        ],
      ),
      input.labeled_input(
        "Password",
        "password",
        form.field_error_messages(m.form, "password"),
        [
          attribute.type_("password"),
          attribute.placeholder("••••••••"),
          attribute.autocomplete("new-password"),
          attribute.disabled(m.loading),
        ],
      ),
      input.labeled_input(
        "Confirm Password",
        "confirm_password",
        form.field_error_messages(m.form, "confirm_password"),
        [
          attribute.type_("password"),
          attribute.placeholder("••••••••"),
          attribute.autocomplete("new-password"),
          attribute.disabled(m.loading),
        ],
      ),
    ]),
    button.button(
      case m.loading {
        True -> "Creating account..."
        False -> "Create Account"
      },
      [
        button.primary(),
        attribute.type_("submit"),
        attribute.class("w-full py-2.5"),
        attribute.disabled(m.loading),
      ],
    ),
  ])
}

fn view_full_bleed(m: Model, submit) {
  html.div(
    [
      attribute.class("min-h-screen flex items-center justify-center px-4"),
    ],
    [
      html.div([attribute.class("w-full max-w-sm flex flex-col gap-5")], [
        html.div([attribute.class("flex flex-col gap-0")], [
          html.p(
            [
              attribute.class(
                "font-[DM_Serif_Display,serif] text-2xl text-zinc-300",
              ),
            ],
            [element.text("Create an account on")],
          ),
          html.h1(
            [
              attribute.class(
                "font-[DM_Serif_Display,serif] text-5xl text-pink-400",
              ),
            ],
            [element.text("Tankobon")],
          ),
        ]),
        html.div([attribute.class("flex flex-col gap-5")], [
          case m.error {
            True -> error_banner()
            False -> element.none()
          },
          form_fields(m, submit),
        ]),
      ]),
    ],
  )
}
