import gleam/list
import lustre/attribute
import lustre/element
import lustre/element/html
import tankobon/ui/display

pub fn input(attrs: List(attribute.Attribute(msg))) -> element.Element(msg) {
  html.input([
    attribute.class(
      "px-4 py-2.5 rounded-lg bg-zinc-800 text-white placeholder-zinc-600 border border-zinc-700 focus:border-pink-500 focus:outline-none focus:ring-1 focus:ring-pink-500/20 transition text-sm disabled:opacity-50 disabled:cursor-not-allowed",
    ),
    ..attrs
  ])
}

pub fn labeled_input(
  label: String,
  name: String,
  errors: List(String),
  attrs: List(attribute.Attribute(msg)),
) -> element.Element(msg) {
  html.div([attribute.class("flex flex-col gap-1.5")], [
    display.field_label(label, [attribute.for(name)]),
    input([attribute.id(name), attribute.name(name), ..attrs]),
    html.small(
      [attribute.class("text-red-400 text-xs")],
      errors |> list.map(element.text),
    ),
  ])
}

pub fn checkbox(attrs: List(attribute.Attribute(msg))) -> element.Element(msg) {
  html.input([
    attribute.type_("checkbox"),
    attribute.class(
      "w-4 h-4 rounded border border-zinc-600 bg-zinc-800 accent-pink-500 cursor-pointer outline-none",
    ),
    ..attrs
  ])
}
