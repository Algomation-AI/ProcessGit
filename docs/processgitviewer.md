# ProcessGit Directory Viewer Manifest

ProcessGit viewers are enabled by placing a `processgit.viewer.json` manifest next to the files they control. The viewer only activates when the currently viewed file matches a binding’s `primary_pattern` and the manifest passes validation.

## Manifest location

* The manifest **must** live in the same directory as the files it references.
* The file name is always `processgit.viewer.json`.

## Matching rules

* Bindings are evaluated in order. The first matching `primary_pattern` wins.
* Patterns follow Go `path.Match` semantics.
* If the pattern contains a `/`, it is matched against the full repo-relative path.
* Otherwise it is matched against the file basename.

## Security note

Viewer HTML is trusted and runs without sandboxing in v1. The backend performs best-effort allowlist checks, but a trusted viewer can still bypass them. Treat manifests as privileged configuration until sandboxing is introduced.

## Example manifest

```json
{
  "version": 1,
  "viewers": [
    {
      "id": "vdvc-register",
      "primary_pattern": "vdvc-register.xml",
      "type": "html",
      "entry": "vdvc-register-admin.with-toggle-save.html",
      "edit_allow": ["vdvc-register.xml"],
      "targets": {
        "xsd": "vdvc-register.xsd"
      }
    },
    {
      "id": "other-register",
      "primary_pattern": "*-register.xml",
      "type": "html",
      "entry": "register-admin.html",
      "edit_allow": ["${PRIMARY}"]
    }
  ]
}
```

`edit_allow` must list explicit repo-relative paths in v1. No `${PRIMARY}` substitution is performed.
