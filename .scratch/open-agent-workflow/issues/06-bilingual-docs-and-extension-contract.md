# 06 - Bilingual documentation and adapter extension contract

**What to build:** Publish a complete English and Chinese explanation of OAW,
its evidence-bounded workflow comparison, safe operating model, supported
adapters, and the contract contributors follow to extend it.

**Blocked by:** 01 - Installer foundation and check command; 02 - Claude
user-scope lifecycle; 03 - Remaining core user adapters; 04 - Project-scope
core and extension adapters; 05 - Drift, backups, force, and filesystem
hardening.

**Status:** ready-for-agent

- [ ] `README.md` and `README-zh.md` provide equivalent, complete entry points
      covering background, problems solved, capabilities, quick start,
      lifecycle profiles, support matrix, safety model, and project status.
- [ ] The three-family comparison explains criteria, limitations, the six
      stage scores, full-family choices, and the exact Matt-Superpowers hybrid
      ownership map without presenting the scores as universal benchmarks.
- [ ] Architecture documentation explains the canonical policy, adapters,
      state and backup model, marker semantics, lifecycle lock, and data flow.
- [ ] Adapter documentation records official paths, precedence/import/reload
      behavior, support levels, source URLs, and retrieval dates for all nine
      tools.
- [ ] Installer, lifecycle, security, troubleshooting, and contributing guides
      match the implemented CLI and include recoverable examples for drift,
      forced operations, updates, and uninstall.
- [ ] The extension contract defines target metadata, scopes, rendering,
      collision handling, fixtures, security checks, and the evidence required
      before an adapter becomes supported.
- [ ] Apache-2.0 licensing and contribution files are present, and no document
      claims OAW installs providers, publishes a remote repository, or owns
      third-party workflow content.
- [ ] Documentation links and bilingual navigation pass automated repository
      checks.

