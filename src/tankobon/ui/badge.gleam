import lustre/attribute
import lustre/element
import lustre/element/html

pub fn badge(
  label: String,
  attrs: List(attribute.Attribute(msg)),
) -> element.Element(msg) {
  html.span(
    [
      attribute.class("px-2 py-1 rounded text-xs font-normal flex-shrink-0"),
      ..attrs
    ],
    [element.text(label)],
  )
}

pub fn pending() -> attribute.Attribute(msg) {
  attribute.class("bg-zinc-700 text-zinc-300")
}

pub fn running() -> attribute.Attribute(msg) {
  attribute.class("bg-blue-500/20 text-blue-400")
}

pub fn completed() -> attribute.Attribute(msg) {
  attribute.class("bg-green-500/20 text-green-400")
}

pub fn failed() -> attribute.Attribute(msg) {
  attribute.class("bg-red-500/20 text-red-400")
}

pub fn warning() -> attribute.Attribute(msg) {
  attribute.class("bg-orange-500/20 text-orange-400")
}

pub fn processing() -> attribute.Attribute(msg) {
  attribute.class("bg-violet-500/20 text-violet-400")
}
