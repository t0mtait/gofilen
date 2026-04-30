## [1.1.3](https://github.com/t0mtait/gofilen/compare/v1.1.2...v1.1.3) (2026-04-30)


### Bug Fixes

* **llm:** add HTTP timeout and send explicit done event to prevent TUI hang ([5b8a754](https://github.com/t0mtait/gofilen/commit/5b8a754f38c50f85851a05de31a563d08dc0de5d))
* remove duplicate user/pass fields in WebDAVFiler struct ([8bf82c6](https://github.com/t0mtait/gofilen/commit/8bf82c61a30950b72a46edee5be0d87a7e294c97))

## [1.1.2](https://github.com/t0mtait/gofilen/compare/v1.1.1...v1.1.2) (2026-04-15)


### Bug Fixes

* add ListFiles method to WebDAVFiler (missing method was causing build failure) ([d40b363](https://github.com/t0mtait/gofilen/commit/d40b363f24dbcfe13e22097b1efc9371c6c9c17f))
* add ListFiles structured method to Filer interface and handlers ([8eb79fb](https://github.com/t0mtait/gofilen/commit/8eb79fb9848da5e4aab478168e3910a7f15cb133))
* bugfix ([1cde77c](https://github.com/t0mtait/gofilen/commit/1cde77c9705011f19dff02b12fc8e1bd498bec36))

## [1.1.1](https://github.com/t0mtait/gofilen/compare/v1.1.0...v1.1.1) (2026-04-15)


### Bug Fixes

* rewrite embedded web UI - add init, fetch timeouts, file sidebar, proper rendering ([bba341d](https://github.com/t0mtait/gofilen/commit/bba341d2be310be0a6f4ef4273c747b9464aaf19))

# [1.1.0](https://github.com/t0mtait/gofilen/compare/v1.0.1...v1.1.0) (2026-04-14)


### Bug Fixes

* single e.Info() syscall in List and consistent error messages ([8beece7](https://github.com/t0mtait/gofilen/commit/8beece774ca41a6c71853ff0238efd6458557fbd))
* wait for stream events while awaiting destructive tool confirmation ([dd0e85a](https://github.com/t0mtait/gofilen/commit/dd0e85aef456faa1d0e7dc0dae999fece5e84b32))


### Features

* add 'tree' tool to LLM toolset for directory hierarchy browsing ([184973c](https://github.com/t0mtait/gofilen/commit/184973c36df20501c71a886823f4f0a232949cc9))

## [1.0.1](https://github.com/t0mtait/gofilen/compare/v1.0.0...v1.0.1) (2026-04-01)


### Bug Fixes

* patch path traversal vulnerabilities in Copy and Move ([baf6f0d](https://github.com/t0mtait/gofilen/commit/baf6f0d5436ea8378906df42424a08517795910f))

# 1.0.0 (2026-03-11)


### Features

* LLM capabilities. confirmations ([a4b5844](https://github.com/t0mtait/gofilen/commit/a4b58448c88366c1a6a4b4987008f075446c55ea))
