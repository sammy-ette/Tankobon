import lustre/attribute
import lustre/element
import lustre/element/html
import lustre/event
import tankobon/ui/button
import tankobon/ui/input

pub fn delete_modal(
  show: Bool,
  delete_files: Bool,
  on_toggle: msg,
  on_cancel: msg,
  on_confirm: msg,
) -> element.Element(msg) {
  case show {
    False -> element.none()
    True ->
      html.div(
        [
          attribute.class(
            "fixed inset-0 z-50 flex items-center justify-center bg-black/60",
          ),
        ],
        [
          html.div(
            [
              attribute.class(
                "bg-zinc-900 border border-zinc-800 rounded-xl p-6 w-full max-w-sm mx-4 flex flex-col gap-5",
              ),
            ],
            [
              html.h2([attribute.class("text-base font-semibold")], [
                element.text("Remove series?"),
              ]),
              html.label(
                [
                  attribute.class(
                    "flex items-center gap-3 cursor-pointer select-none",
                  ),
                ],
                [
                  input.checkbox([
                    attribute.checked(delete_files),
                    event.on_check(fn(_) { on_toggle }),
                  ]),
                  html.span([attribute.class("text-sm text-zinc-300")], [
                    element.text("Also delete files from disk"),
                  ]),
                ],
              ),
              html.div([attribute.class("flex gap-2 justify-end")], [
                button.button("Cancel", [
                  button.secondary(),
                  event.on_click(on_cancel),
                ]),
                button.button("Remove", [
                  button.danger(),
                  event.on_click(on_confirm),
                ]),
              ]),
            ],
          ),
        ],
      )
  }
}
