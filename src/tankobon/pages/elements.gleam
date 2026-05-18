import gleam/bool
import lustre
import lustre/attribute
import lustre/effect
import lustre/element
import lustre/element/html
import lustre/event
import tankobon/ui/badge
import tankobon/ui/button
import tankobon/ui/display
import tankobon/ui/modal

pub type Model {
  Model(show_modal: Bool, delete_files: Bool)
}

pub type Msg {
  OpenModal
  CloseModal
  ToggleDeleteFiles
}

pub fn register() {
  let app = lustre.component(init, update, view, [])
  lustre.register(app, "elements-page")
}

pub fn element() {
  element.element("elements-page", [], [])
}

fn init(_) {
  #(Model(show_modal: False, delete_files: False), effect.none())
}

fn update(m: Model, msg: Msg) {
  case msg {
    OpenModal -> #(Model(..m, show_modal: True), effect.none())
    CloseModal -> #(Model(..m, show_modal: False), effect.none())
    ToggleDeleteFiles -> #(
      Model(..m, delete_files: m.delete_files |> bool.negate),
      effect.none(),
    )
  }
}

fn view(m: Model) {
  html.div(
    [
      attribute.class(
        "min-h-full bg-zinc-950 text-white flex justify-center px-8 py-8",
      ),
    ],
    [
      modal.delete_modal(
        m.show_modal,
        m.delete_files,
        ToggleDeleteFiles,
        CloseModal,
        CloseModal,
      ),
      html.div([attribute.class("w-full max-w-4xl flex flex-col gap-12")], [
        section("Buttons", [
          subsection("Variants", [
            button.button("Default", []),
            button.button("Secondary", [button.secondary()]),
            button.button("Primary", [button.primary()]),
            button.button("Danger", [button.danger()]),
          ]),
          subsection("Disabled", [
            button.button("Secondary", [
              button.secondary(),
              attribute.disabled(True),
            ]),
            button.button("Primary", [
              button.primary(),
              attribute.disabled(True),
            ]),
            button.button("Danger", [
              button.danger(),
              attribute.disabled(True),
            ]),
          ]),
          subsection("Icon only", [
            button.icon("ph ph-arrow-left", [button.secondary()]),
            button.icon("ph ph-plus", [button.primary()]),
            button.icon("ph ph-trash", [button.danger()]),
            button.icon("ph ph-trash", [
              button.secondary(),
              attribute.disabled(True),
            ]),
          ]),
          subsection("Icon + label", [
            button.icon_label("ph ph-plus", "Add", [button.primary()]),
            button.icon_label("ph ph-trash", "Remove", [button.danger()]),
            button.icon_label("ph ph-arrow-clockwise", "Refresh", [
              button.secondary(),
            ]),
            button.icon_label("ph ph-trash", "Disabled", [
              button.danger(),
              attribute.disabled(True),
            ]),
          ]),
        ]),
        section("Badges", [
          subsection("Variants", [
            badge.badge("Pending", [badge.pending()]),
            badge.badge("Running", [badge.running()]),
            badge.badge("Completed", [badge.completed()]),
            badge.badge("Failed", [badge.failed()]),
            badge.badge("Warning", [badge.warning()]),
          ]),
        ]),
        section("Display", [
          subsection("Loading", [display.loading()]),
          subsection("Section Heading", [
            display.section_heading("Example Heading", []),
          ]),
        ]),
        section("Modal", [
          subsection("Delete Modal", [
            button.button("Open", [
              button.secondary(),
              event.on_click(OpenModal),
            ]),
          ]),
        ]),
      ]),
    ],
  )
}

fn section(title: String, children: List(element.Element(Msg))) {
  html.div([attribute.class("flex flex-col gap-6")], [
    html.h2(
      [attribute.class("text-lg font-semibold border-b border-zinc-800 pb-3")],
      [element.text(title)],
    ),
    html.div([attribute.class("flex flex-wrap gap-8")], children),
  ])
}

fn subsection(title: String, children: List(element.Element(Msg))) {
  html.div([attribute.class("flex flex-col gap-3")], [
    html.p([attribute.class("text-xs text-zinc-500 uppercase tracking-wider")], [
      element.text(title),
    ]),
    html.div([attribute.class("flex flex-wrap items-center gap-2")], children),
  ])
}
