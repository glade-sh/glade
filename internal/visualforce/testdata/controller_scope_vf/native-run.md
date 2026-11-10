Fresh Developer edition scratch org, 2026-11-09. Each controller source and
page were deployed separately at the page and Apex API versions recorded in
org.tsv. Accepted cases were executed through Page.getContent using anonymous
Apex, which asserted that the rendered marker was present. The override cases
exercise controller compilation; their getter returns a marker and does not
invoke the overridden method. The scratch org was deleted successfully and
its subsequent lookup failed.
