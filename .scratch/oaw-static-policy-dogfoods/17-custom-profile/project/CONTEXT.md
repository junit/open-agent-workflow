# Release Manifest Context

This context defines the language used to validate a release manifest.

## Language

**Release Manifest**:
A text document of unique `key=value` fields describing one release.
_Avoid_: Config, environment file

**Required Field**:
A named manifest field that must be present and non-empty for validation.
_Avoid_: Optional setting, hint

**Duplicate Field**:
A key that appears more than once in one manifest; it invalidates the document.
_Avoid_: Override, last-write-wins value
