import lustre/attribute
import lustre/element
import lustre/element/html
import tankobon/ui/badge

pub type ToastKind {
  Success
  Failure
}

pub type Toast {
  Toast(kind: ToastKind, message: String)
}

pub fn view(toast: Toast) -> element.Element(msg) {
  let #(icon, badge_attr, border_class) = case toast.kind {
    Success -> #("ph-fill ph-check-circle", badge.completed(), "border-green-500/30")
    Failure -> #("ph-fill ph-warning-circle", badge.failed(), "border-red-500/30")
  }
  html.div(
    [
      attribute.class(
        "fixed bottom-4 right-4 z-[60] flex items-center gap-3 px-4 py-3 rounded-xl border bg-zinc-900 shadow-lg max-w-sm "
        <> border_class,
      ),
    ],
    [
      html.span([attribute.class("rounded-full p-1.5"), badge_attr], [
        html.i([attribute.class("leading-none text-lg " <> icon)], []),
      ]),
      html.span([attribute.class("text-sm text-zinc-200")], [
        element.text(toast.message),
      ]),
    ],
  )
}
