export default function sanitizeHTML(dirty, _config = undefined) {
  const template = document.createElement("template");
  template.innerHTML = String(dirty || "");
  for (const node of template.content.querySelectorAll("script")) {
    node.remove();
  }
  return template.innerHTML;
}
