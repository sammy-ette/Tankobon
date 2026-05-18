import lustre/attribute
import lustre/element
import lustre/element/html

const label_class = "text-sm font-medium text-zinc-400 uppercase tracking-wider"

pub fn loading() -> element.Element(msg) {
  html.div([attribute.class("text-zinc-500 text-sm")], [
    element.text("Loading..."),
  ])
}

pub fn section_heading(
  title: String,
  attrs: List(attribute.Attribute(msg)),
) -> element.Element(msg) {
  html.h3([attribute.class(label_class), ..attrs], [element.text(title)])
}

pub fn field_label(
  title: String,
  attrs: List(attribute.Attribute(msg)),
) -> element.Element(msg) {
  html.label([attribute.class(label_class), ..attrs], [element.text(title)])
}
