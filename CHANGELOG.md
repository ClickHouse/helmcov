# Changelog

## [0.8.0](https://github.com/ClickHouse/helmcov/compare/v0.7.0...v0.8.0) (2026-11-09)


### ⚠ BREAKING CHANGES

* **instrumentation:** give same-line constructs their own branch keys ([#29](https://github.com/ClickHouse/helmcov/issues/29))

### Features

* **instrumentation:** give same-line constructs their own branch keys ([#29](https://github.com/ClickHouse/helmcov/issues/29)) ([6287006](https://github.com/ClickHouse/helmcov/commit/62870067a198c178dbc7098398d15cde6abb42bd))
* report coverage reached by tests alone ([#30](https://github.com/ClickHouse/helmcov/issues/30)) ([a9efbf8](https://github.com/ClickHouse/helmcov/commit/a9efbf843769a4f1cdc3abdbfe0ae8d78cb3e659))
* show coverage change from the base commit on pull requests ([#31](https://github.com/ClickHouse/helmcov/issues/31)) ([b0e13a8](https://github.com/ClickHouse/helmcov/commit/b0e13a8641aae1d0195effca5699f8f85543f981))


### Bug Fixes

* **action:** expose the coverage outputs ([#28](https://github.com/ClickHouse/helmcov/issues/28)) ([6850ca7](https://github.com/ClickHouse/helmcov/commit/6850ca76554207afa9a9b97f2621b883b9a491ad))
* render unpacked subchart templates with the subchart's context ([#27](https://github.com/ClickHouse/helmcov/issues/27)) ([a2d6900](https://github.com/ClickHouse/helmcov/commit/a2d6900048c804caafaea4b9d4e3b1d2c23507fe))

## [0.7.0](https://github.com/ClickHouse/helmcov/compare/v0.6.0...v0.7.0) (2026-11-05)


### Features

* **cli:** add --branch-threshold and the branch-threshold action input ([#25](https://github.com/ClickHouse/helmcov/issues/25)) ([c08e712](https://github.com/ClickHouse/helmcov/commit/c08e7123c951e3a0afc31624bcf57d5c4153ff81))


### Bug Fixes

* **cli:** stop test renders at fail, keep coverage for expected failures ([#24](https://github.com/ClickHouse/helmcov/issues/24)) ([a2c6683](https://github.com/ClickHouse/helmcov/commit/a2c6683bf5d343f45eef43dea64090a2665d116f))

## [0.6.0](https://github.com/ClickHouse/helmcov/compare/v0.5.2...v0.6.0) (2026-11-05)


### ⚠ BREAKING CHANGES

* **chartloader:** suites and tests now render with helm-unittest's defaults (RELEASE-NAME, NAMESPACE, Kubernetes 1.20, no API versions) instead of helm template's. Charts that branch on .Capabilities.APIVersions.Has must list those versions under capabilities.apiVersions. --kube-version and the action's kube-version input now default to empty, meaning 1.20.
* **instrumentation:** branches written as `else if` / `else with` now report as `else-if` / `else-with`, and the new and/or, default, ternary and coalesce branches lower branch coverage for most charts. Line coverage and --threshold are unchanged.

### Features

* **chartloader:** answer lookup from helm-unittest's kubernetesProvider ([#23](https://github.com/ClickHouse/helmcov/issues/23)) ([675e1e0](https://github.com/ClickHouse/helmcov/commit/675e1e00634fbea5d61755220b8f508f3f5d5f36))
* **chartloader:** render each helm-unittest test with its own values ([#15](https://github.com/ClickHouse/helmcov/issues/15)) ([5e208cb](https://github.com/ClickHouse/helmcov/commit/5e208cb2df01723aa365659f069e26e9227a6910))
* **chartloader:** render suites with helm-unittest's context and template globs ([#21](https://github.com/ClickHouse/helmcov/issues/21)) ([74748f5](https://github.com/ClickHouse/helmcov/commit/74748f53b1fc40ecbab93e2b2af932f0efbb3d37))
* **cli:** add --version and issue templates ([#20](https://github.com/ClickHouse/helmcov/issues/20)) ([8b343f8](https://github.com/ClickHouse/helmcov/commit/8b343f8017e368b09fb16eb6883494d91c6accfc))
* **instrumentation:** trace and/or operands, default, ternary, coalesce and else chains ([#16](https://github.com/ClickHouse/helmcov/issues/16)) ([e6b7c84](https://github.com/ClickHouse/helmcov/commit/e6b7c84b99f4f58b04dbb2efb092ce08d90b7de2))


### Bug Fixes

* **action:** verify release downloads, pin actions by SHA, attest releases ([#18](https://github.com/ClickHouse/helmcov/issues/18)) ([c1b6480](https://github.com/ClickHouse/helmcov/commit/c1b64805fe3719a6e3cb1889eb6849ccb7334575))
* **chartloader:** keep values under non-string YAML keys, raise coverage gate to 85% ([#14](https://github.com/ClickHouse/helmcov/issues/14)) ([48faf7c](https://github.com/ClickHouse/helmcov/commit/48faf7cc9ff31dd97784dea9122cc19424d8f841))
* **chartloader:** match Helm's .Files, .helmignore and YAML handling ([#17](https://github.com/ClickHouse/helmcov/issues/17)) ([c0c17c8](https://github.com/ClickHouse/helmcov/commit/c0c17c86d1a2ebadb93a4b01718c5d0272429b88))
* **instrumentation:** match Helm's tpl and quote output, add e2e tests ([#12](https://github.com/ClickHouse/helmcov/issues/12)) ([22093ea](https://github.com/ClickHouse/helmcov/commit/22093ea4bf6235eeb1d6e0505f75ba2b3b4245c8))

## [0.5.2](https://github.com/ClickHouse/helmcov/compare/v0.5.1...v0.5.2) (2026-11-05)


### Bug Fixes

* build with the latest Go release and harden CI checks ([#8](https://github.com/ClickHouse/helmcov/issues/8)) ([6fb20ba](https://github.com/ClickHouse/helmcov/commit/6fb20bac6ef1bec117a33d500810df02d9c834a1))
* **cli:** skip packaged subchart templates listed in suites ([#11](https://github.com/ClickHouse/helmcov/issues/11)) ([bf87226](https://github.com/ClickHouse/helmcov/commit/bf87226103cca9701e1cb6af0c73e35124ca866e))
* **instrumentation:** record coverage while templates execute ([#10](https://github.com/ClickHouse/helmcov/issues/10)) ([c85bb10](https://github.com/ClickHouse/helmcov/commit/c85bb10ee41edf6cecfdd169df3ccd101baa8d1f))

## [0.5.1](https://github.com/ClickHouse/helmcov/compare/v0.5.0...v0.5.1) (2026-11-05)


### Bug Fixes

* publish release binaries from ClickHouse/helmcov ([e9a0ffa](https://github.com/ClickHouse/helmcov/commit/e9a0ffa0b6f3f4f114bd7391469931cc524c22d3))
* publish release binaries from ClickHouse/helmcov ([aff08a2](https://github.com/ClickHouse/helmcov/commit/aff08a2a3cee00d5e2f466f545135792457e34a9))

## [0.5.0](https://github.com/ClickHouse/helmcov/compare/v0.4.3...v0.5.0) (2026-11-05)


### Features

* **ci:** hardened CI workflows to ensure formatting, etc runs ([#4](https://github.com/ClickHouse/helmcov/issues/4)) ([691baf6](https://github.com/ClickHouse/helmcov/commit/691baf6bda253ea27b2dc5c788f3f8b0c09af9c0))
* **ci:** publish GitHub Action with PR coverage comments ([#16](https://github.com/ClickHouse/helmcov/issues/16)) ([6d6b38f](https://github.com/ClickHouse/helmcov/commit/6d6b38f419e18781bc11af81ecabff596ec75fb6))
* **ci:** publish GitHub Action with PR coverage comments ([#16](https://github.com/ClickHouse/helmcov/issues/16)) ([db68eaa](https://github.com/ClickHouse/helmcov/commit/db68eaa277d29104c2e333fa778fdb17b687c81a))
* **helm:** add Helm rendering support and tpl branch tracing ([#6](https://github.com/ClickHouse/helmcov/issues/6)) ([8c18c6c](https://github.com/ClickHouse/helmcov/commit/8c18c6cce5b341232fdd611ee5eaf700bbf01d89))


### Bug Fixes

* action version pinned to latest ([#22](https://github.com/ClickHouse/helmcov/issues/22)) ([11c58e0](https://github.com/ClickHouse/helmcov/commit/11c58e05b24dd3696c72c25a96c47ff02d4e3321))
* action version pinned to latest ([#22](https://github.com/ClickHouse/helmcov/issues/22)) ([197cc79](https://github.com/ClickHouse/helmcov/commit/197cc79935a863b21a9c08fe98fbd959a26af3ae))
* **ci:** default action binary to v0.4.0 for markdown support ([#18](https://github.com/ClickHouse/helmcov/issues/18)) ([b7901f2](https://github.com/ClickHouse/helmcov/commit/b7901f2826a532d7a603bb2f7bbb7bc3fb8d74df))
* **ci:** default action binary to v0.4.0 for markdown support ([#18](https://github.com/ClickHouse/helmcov/issues/18)) ([a1919d0](https://github.com/ClickHouse/helmcov/commit/a1919d06f4d7cf69b94e9c30d639bed33d8804b9))
* Fixing valuegen issues ([#9](https://github.com/ClickHouse/helmcov/issues/9)) ([1b1173f](https://github.com/ClickHouse/helmcov/commit/1b1173fa59655e0778dfc9ac7b7fbfdc72c3c668))
* **release:** fix CI steps for initial release ([9df94e4](https://github.com/ClickHouse/helmcov/commit/9df94e4c41c349edf6c2a2ebd5e230d0e513730d))
* **release:** fix CI steps for initial release ([88b6fe5](https://github.com/ClickHouse/helmcov/commit/88b6fe5bf619d70748e2bd01cdb91952f3a9bd2c))
* **release:** use runtime dockerfile for goreleaser images ([907e261](https://github.com/ClickHouse/helmcov/commit/907e2612ec43c04a023be67b14b20d51e74ec062))
* **report:** simplify PR comment markdown output ([#20](https://github.com/ClickHouse/helmcov/issues/20)) ([cbe132f](https://github.com/ClickHouse/helmcov/commit/cbe132fe502dcb8d7ca17ff94ca4890a8d890a21))
* **report:** simplify PR comment markdown output ([#20](https://github.com/ClickHouse/helmcov/issues/20)) ([6f3f89e](https://github.com/ClickHouse/helmcov/commit/6f3f89e75f8b4136784c05f3c0d10dcafe7ce648))
* resolve coverage calculations ([#14](https://github.com/ClickHouse/helmcov/issues/14)) ([045c249](https://github.com/ClickHouse/helmcov/commit/045c249e41d828d5947115f7bf73ae98cc1ed814))
* resolving CI issues ([#12](https://github.com/ClickHouse/helmcov/issues/12)) ([090f340](https://github.com/ClickHouse/helmcov/commit/090f3409f498c7066e9d1249ae15982c9e3bc701))

## [0.4.3](https://github.com/jordan-simonovski/helmcov/compare/v0.4.2...v0.4.3) (2026-06-27)


### Bug Fixes

* action version pinned to latest ([#22](https://github.com/jordan-simonovski/helmcov/issues/22)) ([197cc79](https://github.com/jordan-simonovski/helmcov/commit/197cc79935a863b21a9c08fe98fbd959a26af3ae))

## [0.4.2](https://github.com/jordan-simonovski/helmcov/compare/v0.4.1...v0.4.2) (2026-06-27)


### Bug Fixes

* **report:** simplify PR comment markdown output ([#20](https://github.com/jordan-simonovski/helmcov/issues/20)) ([6f3f89e](https://github.com/jordan-simonovski/helmcov/commit/6f3f89e75f8b4136784c05f3c0d10dcafe7ce648))

## [0.4.1](https://github.com/jordan-simonovski/helmcov/compare/v0.4.0...v0.4.1) (2026-06-27)


### Bug Fixes

* **ci:** default action binary to v0.4.0 for markdown support ([#18](https://github.com/jordan-simonovski/helmcov/issues/18)) ([a1919d0](https://github.com/jordan-simonovski/helmcov/commit/a1919d06f4d7cf69b94e9c30d639bed33d8804b9))

## [0.4.0](https://github.com/jordan-simonovski/helmcov/compare/v0.3.3...v0.4.0) (2026-06-27)


### Features

* **ci:** publish GitHub Action with PR coverage comments ([#16](https://github.com/jordan-simonovski/helmcov/issues/16)) ([db68eaa](https://github.com/jordan-simonovski/helmcov/commit/db68eaa277d29104c2e333fa778fdb17b687c81a))

## [0.3.3](https://github.com/jordan-simonovski/helmcov/compare/v0.3.2...v0.3.3) (2026-06-27)


### Bug Fixes

* resolve coverage calculations ([#14](https://github.com/jordan-simonovski/helmcov/issues/14)) ([045c249](https://github.com/jordan-simonovski/helmcov/commit/045c249e41d828d5947115f7bf73ae98cc1ed814))

## [0.3.2](https://github.com/jordan-simonovski/helmcov/compare/v0.3.1...v0.3.2) (2026-06-27)


### Bug Fixes

* resolving CI issues ([#12](https://github.com/jordan-simonovski/helmcov/issues/12)) ([090f340](https://github.com/jordan-simonovski/helmcov/commit/090f3409f498c7066e9d1249ae15982c9e3bc701))

## [0.3.1](https://github.com/jordan-simonovski/helmcov/compare/v0.3.0...v0.3.1) (2026-06-27)


### Bug Fixes

* Fixing valuegen issues ([#9](https://github.com/jordan-simonovski/helmcov/issues/9)) ([1b1173f](https://github.com/jordan-simonovski/helmcov/commit/1b1173fa59655e0778dfc9ac7b7fbfdc72c3c668))

## [0.3.0](https://github.com/jordan-simonovski/helmcov/compare/v0.2.0...v0.3.0) (2026-06-27)


### Features

* **helm:** add Helm rendering support and tpl branch tracing ([#6](https://github.com/jordan-simonovski/helmcov/issues/6)) ([8c18c6c](https://github.com/jordan-simonovski/helmcov/commit/8c18c6cce5b341232fdd611ee5eaf700bbf01d89))

## [0.2.0](https://github.com/jordan-simonovski/helmcov/compare/v0.1.3...v0.2.0) (2026-03-12)


### Features

* **ci:** hardened CI workflows to ensure formatting, etc runs ([#4](https://github.com/jordan-simonovski/helmcov/issues/4)) ([691baf6](https://github.com/jordan-simonovski/helmcov/commit/691baf6bda253ea27b2dc5c788f3f8b0c09af9c0))

## [0.1.3](https://github.com/jordan-simonovski/helmcov/compare/v0.1.2...v0.1.3) (2026-03-12)


### Bug Fixes

* **release:** fix CI steps for initial release ([9df94e4](https://github.com/jordan-simonovski/helmcov/commit/9df94e4c41c349edf6c2a2ebd5e230d0e513730d))

## [0.1.2](https://github.com/jordan-simonovski/helmcov/compare/v0.1.1...v0.1.2) (2026-03-12)


### Bug Fixes

* **release:** fix CI steps for initial release ([88b6fe5](https://github.com/jordan-simonovski/helmcov/commit/88b6fe5bf619d70748e2bd01cdb91952f3a9bd2c))
* **release:** use runtime dockerfile for goreleaser images ([907e261](https://github.com/jordan-simonovski/helmcov/commit/907e2612ec43c04a023be67b14b20d51e74ec062))

## [0.1.1](https://github.com/jordan-simonovski/helmcov/compare/helmcov-v0.1.0...helmcov-v0.1.1) (2026-03-12)


### Bug Fixes

* **release:** use runtime dockerfile for goreleaser images ([907e261](https://github.com/jordan-simonovski/helmcov/commit/907e2612ec43c04a023be67b14b20d51e74ec062))

## Changelog

All notable changes to this project are documented in this file.

This file is managed by Release Please.
