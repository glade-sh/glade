# Site and Documentation Feedback Design

The August 12 review is the approved design. This change keeps the current
Glade visual system and implements the shortest coherent user journey:

1. Generated release and build metadata establish which artifact and commit the
   site describes.
2. The homepage leads with one local result, one first-check action, and one real
   VS Code product image.
3. Install verifies the binary with `glade version`; the canonical quickstart
   establishes Salesforce DX project context before `glade doctor --project .`.
4. The first result routes readers to job-based workflows, exact reference, or
   symptom-based troubleshooting.
5. Dense capability truth remains available behind a generated, searchable
   summary and the full checked ledgers.

The existing VitePress theme, generated editor-support catalog, checked help
screenshots, route manifest, and browser test matrix remain the implementation
base. No analytics service, new dependency, or decorative dashboard is added.

Release truth is represented by a checked copy of the public stable release
manifest. The site build emits `/site-build.json` from that manifest and the
deployment commit. Post-deploy smoke compares that endpoint with the live
manifest, GitHub latest release, homepage link, checksums, and advertised
assets. A prerelease branch can carry a different checked manifest without
presenting it as stable until the sync/check gate passes.

The public information architecture has one canonical sidebar location per
user-facing surface. Maintainer content stays separate and noindex. Successful
procedures are Task guides; recovery starts from a Troubleshooting page keyed by
recognizable symptoms. Protocol and contributor detail remain deep links rather
than first-run navigation.

Targeted tests cover the release object, content lint, first-run ordering,
navigation uniqueness, dynamic filter announcements, product-image alt text,
and 320-360px reflow. Existing route, accessibility, build, preview, and
post-deploy checks remain authoritative.
